package diff

import (
	"context"
	"errors"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/omnigres/cli/orb"
	"io"
	"os"
)

func Migra(ctx context.Context, cluster orb.OrbCluster, fromDb, toDb string) (err error) {
	var cli *client.Client
	cli, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())

	switch c := cluster.(type) {
	case *orb.DockerOrbCluster:
		var networkId string
		networkId, err = c.NetworkID(ctx)
		if err != nil {
			return
		}
		var networkIp string
		networkIp, err = c.NetworkIP(ctx)
		if err != nil {
			return
		}

		imageName := "supabase/migra:3.0.1663481299"

		// Try getting the image locally
		_, _, err = cli.ImageInspectWithRaw(ctx, imageName)

		if err != nil && !errdefs.IsNotFound(err) {
			return
		}

		if errdefs.IsNotFound(err) {
			// Pull the image
			var reader io.ReadCloser
			reader, err = cli.ImagePull(ctx, imageName, image.PullOptions{})
			if err != nil {
				return
			}
			defer reader.Close()
			io.Copy(os.Stdout, reader)
		}

		hostconfig := container.HostConfig{
			NetworkMode: container.NetworkMode(networkId),
		}

		// Create container
		var containerResponse container.CreateResponse
		command := []string{"migra",
			"postgresql://omnigres:omnigres@" + networkIp + "/" + fromDb,
			"postgresql://omnigres:omnigres@" + networkIp + "/" + toDb,
			"--unsafe",
		}
		containerResponse, err = cli.ContainerCreate(ctx, &container.Config{
			Image: imageName,
			Cmd:   command,
			Tty:   false,
		}, &hostconfig, nil, nil, "")
		if err != nil {
			return
		}

		var resp types.HijackedResponse
		resp, err = cli.ContainerAttach(ctx, containerResponse.ID, container.AttachOptions{
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

		// Connect stdin to the terminal
		go func() {
			_, _ = io.Copy(resp.Conn, os.Stdin)
		}()

		// Connect stdout/stderr to the terminal
		go func() {
			_, _ = stdcopy.StdCopy(os.Stdout, os.Stderr, resp.Reader)
		}()

		// Start container
		err = cli.ContainerStart(ctx, containerResponse.ID, container.StartOptions{})
		if err != nil {
			return
		}

		statusCh, errCh := cli.ContainerWait(ctx, containerResponse.ID, container.WaitConditionNotRunning)
		select {
		case err := <-errCh:
			if err != nil {
				panic(err)
			}
		case status := <-statusCh:
			if status.StatusCode == 0 {
				fmt.Printf("migra exited with status: %d\n", status.StatusCode)
			}
		}
	default:
		err = errors.New("Unsupported Orb cluster type")
	}
	return
}
