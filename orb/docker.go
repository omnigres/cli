package orb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"os/user"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	_ "github.com/lib/pq"
	"github.com/omnigres/cli/internal/fileutils"
	"github.com/omnigres/cli/tui"
	"github.com/spf13/viper"
	"golang.org/x/term"
)

const default_directory_mount = "/mnt/host"

type DockerOrbCluster struct {
	client             *client.Client
	currentContainerId string
	OrbOptions
}

func (d *DockerOrbCluster) Config() *Config {
	return d.OrbOptions.Config
}

func NewDockerOrbCluster() (orb OrbCluster, err error) {
	log.Debugf(
		"Creating docker client from env."+
			"\n %s: %s"+
			"\n %s: %s"+
			"\n %s: %s"+
			"\n %s: %s",
		client.EnvOverrideHost,
		os.Getenv(client.EnvOverrideHost),
		client.EnvOverrideAPIVersion,
		os.Getenv(client.EnvOverrideAPIVersion),
		client.EnvOverrideCertPath,
		os.Getenv(client.EnvOverrideCertPath),
		client.EnvTLSVerify,
		os.Getenv(client.EnvTLSVerify),
	)
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return
	}
	orb = &DockerOrbCluster{client: cli, OrbOptions: OrbOptions{}}
	return
}

func (d *DockerOrbCluster) Configure(options OrbOptions) error {
	d.OrbOptions = options
	return nil
}

func (d *DockerOrbCluster) prepareImage(ctx context.Context) (digest string, err error) {
	cli := d.client
	imageName := d.Config().Image.Name

	var img types.ImageInspect

	// Try getting the image locally
	img, _, err = cli.ImageInspectWithRaw(ctx, imageName)

	notFound := errdefs.IsNotFound(err)

	// If there's an error and if it is not "not found" error, propagate it
	if err != nil && !notFound {
		return
	}

	digest = imageName

	if !notFound {
		// Get the digest (if found)
		if len(img.RepoDigests) > 0 {
			digest = img.RepoDigests[0]

			// If digest does not match, it is as good as if was not found
			if d.Config().Image.Digest != "" && d.Config().Image.Digest != digest {
				imageName = digest
				notFound = true
			}
		}
	}

	if notFound {
		// Pull the image
		var reader io.ReadCloser
		reader, err = cli.ImagePull(ctx, imageName, image.PullOptions{})
		if err != nil {
			return
		}
		defer reader.Close()

		progress := tui.NewDownloadProgress("Downloading docker image "+imageName, reader)
		progressModel, progressError := progress.Run()
		if progressError != nil {
			fmt.Println("Download failed to run for image", imageName, progressError.Error())
			os.Exit(1)
		}

		m := progressModel.(tui.Model)
		if m.Err != nil {
			fmt.Println("Oh no! Could not download image", imageName)
			os.Exit(1)
		}

		// Getting the image locally again to get the digest
		img, _, err = cli.ImageInspectWithRaw(ctx, imageName)
		if err != nil {
			return
		}

		// Fetch the digest
		if len(img.RepoDigests) > 0 {
			digest = img.RepoDigests[0]
		}
	}

	// Ensure the config has been updated
	if d.Config().Image.Name != digest {
		d.Config().Image.Digest = digest
	}

	return

}

func (d *DockerOrbCluster) runfile() (v *viper.Viper) {
	v = viper.New()
	v.SetConfigFile(d.Path + "/omnigres.run.yaml")
	return
}

func (d *DockerOrbCluster) waitUntilClusterIsReady(ctx context.Context, listeners []OrbStartEventListener, cancel context.CancelFunc) {

	log.Debug("Waiting for is_omnigres_ready...")
	deadline := time.Now().Add(1 * time.Minute)

	ready := false

checkPg:
	for time.Now().Before(deadline) {
		c, err := d.Connect(ctx)
		if err == nil {
			if err = c.Ping(); err != nil {
				continue checkPg
			}
		checkOmnigres:
			for time.Now().Before(deadline) {
				if err = c.QueryRowContext(ctx, "select is_omnigres_ready()").Scan(&ready); err != nil {
					time.Sleep(1 * time.Second)
					log.Debugf("Error trying is_omnigres_ready: %s", err)
					continue checkOmnigres
				}
				_ = c.Close()
				log.Debugf("is_omnigres_ready: %t", ready)
				if ready {
					for _, listener := range listeners {
						if listener.Ready != nil {
							go listener.Ready(d)
						}
					}
					return
				}
				time.Sleep(1 * time.Second)
			}
		}
		time.Sleep(1 * time.Second)
	}

	fmt.Println("Can't get a healthy cluster, terminating...")
	cancel()
}

func (d *DockerOrbCluster) StartWithCurrentUser(ctx context.Context, options OrbClusterStartOptions) (err error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Get the current user
	var currentUser *user.User
	currentUser, err = user.Current()
	if err != nil {
		log.Fatalf("Could not get current user: %s", err)
	}

	err = d.Start(
		ctx,
		options,
		&currentUser.Uid,
		nil,
	)
	if err != nil {
		log.Fatal("Fail starting Orb", "err", err)
	}
	return
}

func (d *DockerOrbCluster) Start(ctx context.Context, options OrbClusterStartOptions, runAs *string, entryPoint []string) (err error) {
	cli := d.client
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var imageDigest string

	var run *viper.Viper
	var containerId string

	if options.Runfile {
		run = d.runfile()
		err = fileutils.CreateIfNotExists(run.ConfigFileUsed(), false)
		if err != nil {
			return
		}

		err = run.ReadInConfig()
		if err != nil {
			return
		}

		containerId, err = d.containerId()
	}

	// Prepare image
	imageDigest, err = d.prepareImage(ctx)
	if err != nil {
		return
	}

checkContainer:
	if containerId != "" {
		log.Debugf("Found a container id %s", containerId)
		var cnt types.ContainerJSON
		cnt, err = cli.ContainerInspect(ctx, containerId)
		if errdefs.IsNotFound(err) {
			log.Warn("Container not found, starting new one", "container", containerId)
			containerId = ""
			goto checkContainer
		}
		if err != nil {
			return
		}
		// Check the container
		if cnt.State.Running {
			err = errors.New("Container already running")
			return
		}

		// Check the image
		var image types.ImageInspect
		image, _, err = cli.ImageInspectWithRaw(ctx, cnt.Image)
		if err != nil {
			return
		}
		if len(image.RepoDigests) > 0 && image.RepoDigests[0] != imageDigest {
			err = fmt.Errorf("Container's image %s does not match expected %s", image.RepoDigests[0], imageDigest)
			return
		}

	} else {

		networkName := "omnigres"

		_, err = cli.NetworkCreate(ctx, networkName, network.CreateOptions{
			Driver: "bridge",
		})

		if err != nil {
			// If it is a conflict, this is normal flow – network already exists
			if !errdefs.IsConflict(err) {
				// otherwise, it's an error
				return
			}
		}

		// Bindings
		hostconfig := container.HostConfig{
			AutoRemove: options.AutoRemove,
			Mounts: []mount.Mount{
				{
					Type:   mount.TypeBind,
					Source: d.Path,
					Target: default_directory_mount,
				},
			},
			NetworkMode: container.NetworkMode(networkName),
		}

		// Prepare environment for every orb
		env := make([]string, 0)
		for _, orb := range d.Config().Orbs {
			for _, e := range os.Environ() {
				if strings.HasPrefix(e, strings.ToUpper(orb.Name+"_")) {
					env = append(env, e)
				}
			}
		}
		env = append(env, "POSTGRES_HOST_AUTH_METHOD=password")
		// Allows to prevent problems with initialization scripts failing due to
		// be unable to chmod /var/lib/postgresql/data (since it already exists
		// and not owned by user passed in `runAs`)
		env = append(env, "PGDATA=/var/lib/postgresql/omnigres")

		// Create container
		log.Debugf("Creating container ...")
		var containerResponse container.CreateResponse
		var config *container.Config
		config = &container.Config{Image: imageDigest, Env: env}
		if runAs != nil {
			log.Debugf("🪪 Starting cluster with current user id: %s", *runAs)
			// Ensure we have the right user and group
			config.User = fmt.Sprintf("%s:postgres", *runAs)
		}
		if entryPoint != nil {
			log.Debugf("🛂 Starting cluster with custom entry point: %s", entryPoint)
			config.Entrypoint = entryPoint
		}
		containerResponse, err = cli.ContainerCreate(
			ctx,
			config,
			&hostconfig,
			nil,
			nil,
			"",
		)
		if err != nil {
			return
		}
		containerId = containerResponse.ID
		d.currentContainerId = containerId
	}

	if options.Attachment.ShouldAttach {
		var resp types.HijackedResponse
		resp, err = cli.ContainerAttach(ctx, containerId, container.AttachOptions{
			Stream: true,
			Stdin:  true,
			Stdout: true,
			Stderr: true,
		})
		if err != nil {
			fmt.Printf("Error attaching to attach instance: %v\n", err)
			return
		}
		defer resp.Close()

		d.currentContainerId = containerId

		// Connect stdout/stderr to the consumer
		for _, listener := range options.Attachment.Listeners {
			if listener.OutputHandler != nil {
				listener.OutputHandler(d, resp.Reader)
			}
		}
	}

	// Start container
	err = cli.ContainerStart(ctx, containerId, container.StartOptions{})
	if err != nil {
		return err
	}

	for _, listener := range options.Listeners {
		if listener.Started != nil {
			go listener.Started(d)
		}
	}

	// If we fail below, stop the container
	defer func() {
		if err != nil || options.Attachment.ShouldAttach {
			timeout := 0 // forcibly terminate
			newErr := cli.ContainerStop(ctx, containerId, container.StopOptions{Timeout: &timeout})

			if newErr != nil {
				err = errors.Join(err, newErr)
			}
			if options.Attachment.ShouldAttach {
				for _, listener := range options.Attachment.Listeners {
					if listener.Stopped != nil {
						go listener.Stopped(d)
					}
				}
			}

		}
	}()

	if options.Runfile {
		run.Set("containerid", containerId)

		err = run.WriteConfig()
		if err != nil {
			return
		}
	}

	// TODO: do this in the background?
	// wait only when we have Listeners
	if options.Listeners != nil {
		d.waitUntilClusterIsReady(ctx, options.Listeners, cancel)
	}

	if options.Attachment.ShouldAttach {
		statusCh, errCh := cli.ContainerWait(ctx, containerId, container.WaitConditionNotRunning)
		sigCtx, stop := signal.NotifyContext(ctx, os.Interrupt)
		defer stop()

		select {
		case <-sigCtx.Done():
			fmt.Println("Terminating cluster")
		case err = <-errCh:
			if err != nil {
				return
			}
		case status := <-statusCh:
			if status.StatusCode == 0 {
				fmt.Printf("Omnigres exited with status: %d\n", status.StatusCode)
			}
		}
	}

	return nil
}

func (d *DockerOrbCluster) containerId() (containerId string, err error) {
	if d.currentContainerId != "" {
		containerId = d.currentContainerId
	} else {
		v := d.runfile()
		err = v.ReadInConfig()
		if err != nil {
			return
		}

		containerId = v.GetString("containerid")
	}
	return
}

func (d *DockerOrbCluster) Stop(ctx context.Context) (err error) {
	cli := d.client

	var id string
	id, err = d.containerId()
	if err != nil {
		return
	}

	var cnt types.ContainerJSON
	cnt, err = cli.ContainerInspect(ctx, id)
	if err != nil {
		return
	}

	if !cnt.State.Running {
		err = errors.New("Container is not running")
		return
	}

	err = cli.ContainerStop(ctx, id, container.StopOptions{})
	if err != nil {
		return
	}
	return
}

func (d *DockerOrbCluster) Close() (err error) {
	err = d.client.Close()
	return
}

func (d *DockerOrbCluster) ConnectPsql(ctx context.Context, database ...string) (err error) {
	var id string
	id, err = d.containerId()
	if err != nil {
		return
	}

	var db string
	if len(database) == 0 {
		db = "omnigres"
	} else {
		db = database[0]
	}
	if len(database) > 1 {
		err = errors.New("orb: database name is ambiguous")
		return
	}
	cli := d.client

	var execResponse types.IDResponse
	execResponse, err = cli.ContainerExecCreate(ctx, id, container.ExecOptions{
		Cmd:          []string{"psql", "-Uomnigres", "--set", "HISTFILE=.psql_history", db},
		WorkingDir:   default_directory_mount,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
	})

	if err != nil {
		return
	}

	// Attach to the exec instance
	resp, err := cli.ContainerExecAttach(ctx, execResponse.ID, container.ExecAttachOptions{
		Tty: true,
	})
	if err != nil {
		fmt.Printf("Error attaching to exec instance: %v\n", err)
		return
	}
	defer resp.Close()

	// Save the original terminal state
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		fmt.Printf("Error setting terminal to raw mode: %v\n", err)
		return
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	// Connect stdin to the terminal
	go func() {
		_, _ = io.Copy(resp.Conn, os.Stdin)
	}()

	// Connect stdout/stderr to the terminal
	_, _ = io.Copy(os.Stdout, resp.Reader)

	return
}

func (d *DockerOrbCluster) NetworkID(ctx context.Context) (network string, err error) {
	cli := d.client

	var id string
	id, err = d.containerId()
	if err != nil {
		return
	}

	var cnt types.ContainerJSON
	cnt, err = cli.ContainerInspect(ctx, id)
	if err != nil {
		return
	}

	if !cnt.State.Running {
		err = errors.New("Container is not running")
		return
	}

	network = cnt.HostConfig.NetworkMode.NetworkName()
	return
}

func (d *DockerOrbCluster) NetworkIP(ctx context.Context) (ip string, err error) {
	cli := d.client

	var id string
	id, err = d.containerId()
	if err != nil {
		return
	}

	var cnt types.ContainerJSON
	cnt, err = cli.ContainerInspect(ctx, id)
	if err != nil {
		return
	}

	if !cnt.State.Running {
		err = errors.New("Container is not running")
		return
	}

	ip = cnt.NetworkSettings.Networks[cnt.HostConfig.NetworkMode.NetworkName()].IPAddress
	return
}

func (d *DockerOrbCluster) Connect(ctx context.Context, database ...string) (conn *sql.DB, err error) {
	var db string
	if len(database) == 0 {
		db = "omnigres"
	} else {
		db = database[0]
	}
	var ip string
	ip, err = d.NetworkIP(ctx)
	if err != nil {
		return
	}
	port := 5432
	conn, err = sql.Open("postgres", fmt.Sprintf("user=omnigres password=omnigres dbname=%s host=%s port=%d sslmode=disable", db, ip, port))
	return
}

func (d *DockerOrbCluster) Endpoints(ctx context.Context) (endpoints []Endpoint, err error) {
	var addr string
	addr, err = d.NetworkIP(ctx)
	if err != nil {
		return
	}
	ipaddr := net.ParseIP(addr)
	endpoints = make([]Endpoint, 0)
	var conn *sql.DB
	conn, err = d.Connect(ctx)
	if err != nil {
		return
	}
	defer conn.Close()

	var rows *sql.Rows
	// Search for all databases
	rows, err = conn.QueryContext(ctx, `select datname from pg_database where not datistemplate and datname != 'postgres'`)
	if err != nil {
		return
	}
	defer rows.Close()
nextDatabase:
	for rows.Next() {
		var datname string
		if err = rows.Scan(&datname); err != nil {
			return
		}
		// For every database
		var dbconn *sql.DB
		dbconn, err = d.Connect(ctx, datname)
		if err != nil {
			return
		}
		defer dbconn.Close()
		// Add the Postgres service
		endpoints = append(endpoints, Endpoint{Database: datname, IP: ipaddr, Port: 5432, Protocol: "Postgres"})
		// Get the list of HTTP listeners.
		// TODO: in the future, we expect this to be generialized through omni_service
		var portRows *sql.Rows
		portRows, err = dbconn.QueryContext(ctx, "select effective_port from omni_httpd.listeners")
		if err != nil {
			err = nil
			continue nextDatabase
		}
		defer portRows.Close()
		for portRows.Next() {
			var port int
			err = portRows.Scan(&port)
			if err != nil {
				return
			}
			endpoints = append(endpoints, Endpoint{Database: datname, IP: ipaddr, Port: port, Protocol: "HTTP"})
		}

	}
	return
}
