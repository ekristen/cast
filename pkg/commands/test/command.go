package test

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/urfave/cli/v2"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/ekristen/cast/pkg/commands"
	"github.com/ekristen/cast/pkg/common"
	"github.com/ekristen/cast/pkg/config"
)

func Execute(c *cli.Context) error {
	if c.Args().Len() != 1 {
		return fmt.Errorf("expect a single argument")
	}

	if c.Path("dir") != "" {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		if err := os.Chdir(c.Path("dir")); err != nil {
			return err
		}
		defer func(dir string) {
			_ = os.Chdir(dir)
		}(cwd)
	}

	state := c.Args().First()

	cfg, err := config.Load(c.Path("config"))
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	// docker run -it --rm --name="sift-state-${STATE}" -v `pwd`/sift:/srv/salt/sift --cap-add SYS_ADMIN teamdfir/sift-saltstack-tester:${DISTRO} \
	//  salt-call -l debug --local --retcode-passthrough --state-output=mixed state.sls ${STATE} pillar="{sift_user: root}"

	basePath := cwd
	if cfg.Manifest.Base != "" {
		basePath = filepath.Join(basePath, cfg.Manifest.Base)
	}

	name := fmt.Sprintf("cast-test-state-%s", randomString(9))

	/*
		args := []string{
			"run", "-i", "--rm",
			fmt.Sprintf(`--name=%s`, name),
			fmt.Sprintf("--volume=%s:/srv/salt/%s", basePath, cfg.Manifest.Name),
			`--cap-add=SYS_ADMIN`,
			fmt.Sprintf("--platform=%s", c.String("platform")),
			c.String("image"),
			"salt-call", "-l", "debug", "--local", "--retcode-passthrough",
			"--state-output=mixed", "state.sls", state,
		}

		cmd := exec.CommandContext(c.Context, "docker", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			var exitError *exec.ExitError
			if errors.As(err, &exitError) {
				os.Exit(exitError.ExitCode())
			}
			return err
		}
	*/

	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(c.Context)
	defer cancel()

	platform := c.String("platform")
	if len(strings.Split(platform, "/")) != 2 {
		return fmt.Errorf("invalid platform format: %s", platform)
	}

	// Pull the image
	logrus.Info("pulling image (if needed)")
	imageOut, err := dockerClient.ImagePull(ctx, c.String("image"), image.PullOptions{
		Platform: platform,
	})
	if err != nil {
		return err
	}
	defer imageOut.Close()

	// Display image pull progress
	_, err = io.Copy(os.Stdout, imageOut)
	if err != nil {
		return err
	}

	logrus.Info("launching container")
	resp, err := dockerClient.ContainerCreate(ctx, &container.Config{
		Image: c.String("image"),
		Cmd: []string{
			"salt-call", "--local", "--retcode-passthrough",
			"-l", c.String("salt-log-level"),
			"--state-output=mixed", "state.sls", state,
		},
	}, &container.HostConfig{
		AutoRemove: true,
		CapAdd:     []string{"SYS_ADMIN"},
		Binds:      []string{fmt.Sprintf("%s:/srv/salt/%s", basePath, cfg.Manifest.Name)},
	}, nil, &v1.Platform{
		OS:           strings.Split(platform, "/")[0],
		Architecture: strings.Split(platform, "/")[1],
	}, name)
	if err != nil {
		return err
	}

	// Handle Control-C (SIGINT) signal
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		logrus.Info("received signal, stopping container")
		_ = dockerClient.ContainerStop(ctx, resp.ID, container.StopOptions{})
		os.Exit(1)
	}()

	if err := dockerClient.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return err
	}

	// Stream logs in real-time
	logOut, err := dockerClient.ContainerLogs(ctx, resp.ID, container.LogsOptions{ShowStdout: true, ShowStderr: true, Follow: true})
	if err != nil {
		return err
	}
	defer logOut.Close()

	go func() {
		_, _ = stdcopy.StdCopy(os.Stdout, os.Stderr, logOut)
	}()

	statusCh, errCh := dockerClient.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return err
		}
	case <-statusCh:
	}

	return nil
}

const charset = "abcdefghijklmnopqrstuvwxyz0123456789"

func randomString(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func init() {
	flags := []cli.Flag{
		&cli.PathFlag{
			Name:  "config",
			Value: ".cast.yml",
		},
		&cli.PathFlag{
			Name:   "dir",
			Hidden: true,
		},
		&cli.StringFlag{
			Name:    "user",
			Usage:   "The user to install against (cannot be root)",
			EnvVars: []string{"SUDO_USER"},
		},
		&cli.StringFlag{
			Name:  "image",
			Value: "ghcr.io/ekristen/cast-tools/saltstack-tester:24.04-3006",
		},
		&cli.StringFlag{
			Name:  "platform",
			Value: fmt.Sprintf("linux/%s", runtime.GOARCH),
		},
		&cli.StringFlag{
			Name:  "salt-log-level",
			Value: "debug",
		},
	}

	cmd := &cli.Command{
		Name:   "test-state",
		Usage:  "test a state",
		Flags:  append(flags, commands.GlobalFlags()...),
		Before: commands.GlobalBefore,
		Action: Execute,
	}

	common.RegisterCommand(cmd)
}
