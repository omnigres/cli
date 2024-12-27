package orb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/charmbracelet/log"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/docker/go-connections/nat"
	_ "github.com/lib/pq"
	"github.com/omnigres/cli/internal/fileutils"
	"github.com/spf13/viper"
	"golang.org/x/term"
	"io"
	"os"
	"strconv"
	"strings"
)

const default_directory_mount = "/mnt/host"

type DockerOrbCluster struct {
	client *client.Client
	OrbOptions
}

func (d *DockerOrbCluster) Config() *Config {
	return d.OrbOptions.Config
}

func NewDockerOrbCluster() (orb OrbCluster, err error) {
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
		io.Copy(os.Stdout, reader)

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

func (d *DockerOrbCluster) Start(ctx context.Context) (err error) {
	cli := d.client

	var imageDigest string

	run := d.runfile()
	err = fileutils.CreateIfNotExists(run.ConfigFileUsed(), false)
	if err != nil {
		return
	}

	err = run.ReadInConfig()
	if err != nil {
		return
	}

	var containerId string
	containerId, err = d.containerId()

	// Prepare image
	imageDigest, err = d.prepareImage(ctx)
	if err != nil {
		return
	}

checkContainer:
	if containerId != "" {
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

		// Bindings
		hostconfig := container.HostConfig{
			PortBindings: nat.PortMap{
				"5432/tcp": []nat.PortBinding{{HostIP: "127.0.0.1"}},
				"8080/tcp": []nat.PortBinding{{HostIP: "127.0.0.1"}},
			},
			Mounts: []mount.Mount{
				{
					Type:   mount.TypeBind,
					Source: d.Path,
					Target: default_directory_mount,
				},
			},
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

		// Create container
		var containerResponse container.CreateResponse
		containerResponse, err = cli.ContainerCreate(ctx, &container.Config{Image: imageDigest, Env: env}, &hostconfig, nil, nil, "")
		if err != nil {
			return
		}
		containerId = containerResponse.ID
	}

	// Start container
	err = cli.ContainerStart(ctx, containerId, container.StartOptions{})
	if err != nil {
		return err
	}

	// If we fail below, stop the container
	defer func() {
		if err != nil {
			timeout := 0 // forcibly terminate
			newErr := cli.ContainerStop(context.TODO(), containerId, container.StopOptions{Timeout: &timeout})
			if newErr != nil {
				err = errors.Join(err, newErr)
			}
		}
	}()

	run.Set("containerid", containerId)

	err = run.WriteConfig()
	if err != nil {
		return
	}

	return nil
}

func (d *DockerOrbCluster) containerId() (containerId string, err error) {
	v := d.runfile()
	err = v.ReadInConfig()
	if err != nil {
		return
	}

	containerId = v.GetString("containerid")
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

func (d *DockerOrbCluster) Port(ctx context.Context, name string) (port int, err error) {
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

	bindings := cnt.NetworkSettings.Ports[nat.Port(name)]

	for _, binding := range bindings {
		port, err = strconv.Atoi(binding.HostPort)
		return
	}

	err = fmt.Errorf("port %s not found", name)
	return
}

func (d *DockerOrbCluster) Connect(ctx context.Context, database ...string) (conn *sql.DB, err error) {
	var db string
	if len(database) == 0 {
		db = "omnigres"
	} else {
		db = database[0]
	}
	var port int
	port, err = d.Port(ctx, "5432/tcp")
	if err != nil {
		return
	}
	conn, err = sql.Open("postgres", fmt.Sprintf("user=omnigres password=omnigres dbname=%s host=127.0.0.1 port=%d sslmode=disable", db, port))
	return
}
