package main

import (
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/golang/glog"
	"github.com/olebedev/config"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

var cfg = loadConfig()

func main() {
	app := cli.NewApp()
	app.Name = "msct"
	app.Version = "1.1.0"
	app.Usage = "Minecraft Server Control Tool"
	app.Author = "Nathan Young (http://github.com/nathanpaulyoung)"
	app.Commands = []cli.Command{
		startCommand(),
		resumeCommand(),
		haltCommand(),
		keepAliveCommand(),
		commandCommand(),
	}
	app.Run(os.Args)
}

func startCommand() cli.Command {
	command := cli.Command{
		Name:    "start",
		Aliases: []string{"s"},
		Usage:   "start a server",
		Action: func(c *cli.Context) {
			servername := c.Args()[0]
			startServer(servername)
		},
	}
	return command
}

func haltCommand() cli.Command {
	command := cli.Command{
		Name:    "halt",
		Aliases: []string{"h", "stop"},
		Usage:   "halt a server",
		Action: func(c *cli.Context) {
			servername := c.Args()[0]
			tmuxSendKeys(servername, 0, 0, "stop")
		},
	}
	return command
}

func resumeCommand() cli.Command {
	command := cli.Command{
		Name:    "resume",
		Aliases: []string{"r"},
		Usage:   "resume a server's tmux session",
		Action: func(c *cli.Context) {
			servername := c.Args()[0]
			args := []string{"a", "-t", buildTmuxName(servername)}
			cmd := exec.Command("tmux", args...)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if serverIsRunning(servername) {
				if err := cmd.Run(); err != nil {
					glog.Fatalf("Error in function resumeCommand(); please inform the developer.\n%s", err)
				}
			} else {
				glog.Errorf("Server '%s' is not running.", servername)
			}
		},
	}
	return command
}

func keepAliveCommand() cli.Command {
	command := cli.Command{
		Name:    "keepalive",
		Aliases: []string{"ka"},
		Usage:   "restart a server's tmux session if server detected as stopped",
		Action: func(c *cli.Context) {
			servername := c.Args()[0]
			keepAliveFreq, _ := cfg.Int("keepAliveFreq")
			for {
				if !serverIsRunning(servername) {
					startServer(servername)
					time.Sleep(time.Second * time.Duration(keepAliveFreq))
				}
			}
		},
	}
	return command
}

func commandCommand() cli.Command {
	command := cli.Command{
		Name:    "command",
		Aliases: []string{"cmd", "c"},
		Usage:   "send a command to the tmux session",
		Action: func(c *cli.Context) {
			servername := c.Args()[0]
			var args []string
			for i := 1; i < len(c.Args()); i++ {
				args = append(args, c.Args()[i])
			}
			command := strings.Join(args, " ")
			tmuxSendKeys(servername, 0, 0, command)
		},
	}
	return command
}

func startServer(servername string) {
	args := buildInvocation(servername)
	cmd := exec.Command("tmux", args...)
	cmd.Dir = buildServerDir(servername)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if serverExists(servername) && !serverIsRunning(servername) {
		if err := cmd.Run(); err != nil {
			glog.Fatalf("Error in function startServer(); please inform the developer.\n%s", err)
		}
	} else {
		glog.Errorf("Server '%s' does not exist or is already running.", servername)
	}
}

func tmuxSendKeys(servername string, window int, pane int, command string) {
	cmd := exec.Command("tmux", "send-keys", "-t", buildTmuxName(servername)+":"+strconv.Itoa(window)+"."+strconv.Itoa(pane), command, "Enter")
	if serverIsRunning(servername) {
		if err := cmd.Run(); err != nil {
			glog.Fatalf("Error in function tmuxSendKeys(); please inform the developer.\n%s", err)
		}
	} else {
		glog.Errorf("Server '%s' is not running.", servername)
	}
}

func serverExists(servername string) bool {
	if _, err := os.Stat(buildServerDir(servername) + getJarFile()); err != nil {
		return false
	}
	return true
}

func serverIsRunning(servername string) bool {
	cmd := exec.Command("tmux", "has-session", "-t", buildTmuxName(servername))
	if err := cmd.Run(); err != nil {
		return false
	}

	return true
}

func loadConfig() *config.Config {
	//Load msct.conf, prefer local over /etc/, and parse yaml
	var yaml *config.Config
	if _, err := os.Stat("./msct.conf"); err == nil {
		yaml, _ = config.ParseYamlFile("./msct.conf")
	} else if _, err := os.Stat("/etc/msct.conf"); err == nil {
		yaml, _ = config.ParseYamlFile("/etc/msct.conf")
	} else {
		err := generateConfig()
		if err != nil {
			glog.Fatalf("Could not generate config file; please inform the developer.\n%s", err)
		}
		glog.Warningf("Could not locate msct.conf, so I generated the default file for you at /etc/msct.conf")
		yaml, _ = config.ParseYamlFile("/etc/msct.conf")
	}

	return yaml
}

func generateConfig() error {
	defaultConfig := map[string]interface{}{
		"user":           "minecraft",
		"screenBaseName": "msct-",
		"rammin":            "2048",
		"rammax":            "4096",
		"paths": map[string]interface{}{
			"root":    "/opt/minecraft/",
			"jarFile": "server.jar",
		},
		"startTmuxAttached": "false",
		"javaParams":        "-XX:+UseConcMarkSweepGC -XX:+UseParNewGC -XX:+CMSParallelRemarkEnabled -XX:ParallelGCThreads=2 -XX:+DisableExplicitGC -XX:MaxGCPauseMillis=500 -XX:SurvivorRatio=16 -XX:TargetSurvivorRatio=90",
		"debug":             "false",
		"keepAliveFreq":     "30",
	}

	yaml, err := config.RenderYaml(defaultConfig)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile("/etc/msct.conf", []byte(yaml), 0644)
	if err != nil {
		return err
	}

	return nil
}

func buildTmuxName(servername string) string {
	//Load from config and set base tmux prefix, if not set in config, default to "msct-"
	tmuxbasename, err := cfg.String("tmuxbasename")
	if err != nil {
		tmuxbasename = "msct-"
	}

	return tmuxbasename + servername
}

func buildInvocation(servername string) []string {
	//Load from config and set whether to start tmux attached or not, if not set in config, default to attached
	startTmuxAttached, err := cfg.Bool("startTmuxAttached")
	if err != nil {
		startTmuxAttached = true
	}
	var tmuxParams []string
	if startTmuxAttached == true {
		tmuxParams = append(tmuxParams, "new", "-s", buildTmuxName(servername))
	} else {
		tmuxParams = append(tmuxParams, "new", "-d", "-s", buildTmuxName(servername))
	}

	//Load from config and set java parameters, if not set in config, set reasonable defaults
	rammin, err := cfg.String("rammin")
	if err != nil {
		rammin = "2048M"
	}

	rammax, err := cfg.String("rammax")
	if err != nil {
		rammax = "4096M"
	}

	//Load from config and set java parameters, if not set in config, set reasonable defaults
	javaParams, err := cfg.String("javaParams")
	if err != nil {
		javaParams = "-XX:+UseConcMarkSweepGC -XX:+UseParNewGC -XX:+CMSParallelRemarkEnabled -XX:ParallelGCThreads=3 -XX:+DisableExplicitGC -XX:MaxGCPauseMillis=500 -XX:SurvivorRatio=16 -XX:TargetSurvivorRatio=90"
	}
	javaParamsArray := strings.Fields(javaParams)

	//Create full server path of the form /opt/minecraft/<servername>/server.jar
	fullpath := buildServerDir(servername) + getJarFile()

	var args []string
	args = append(args, tmuxParams...)
	args = append(args, fmt.Sprintf("java -server -Xms%sM -Xmx%sM %s -jar %s", rammin, rammax, strings.Join(javaParamsArray, " "), fullpath))

	if debugIsEnabled() {
		println(strings.Join(args, " "))
	}

	return args
}

func buildServerDir(servername string) string {
	//Load from config and set msct root directory, if not set in config, default to /opt/minecraft/
	rootdir, err := cfg.String("paths.root")
	if err != nil {
		rootdir = "/opt/minecraft/"
	}

	return rootdir + servername + "/"
}

func getJarFile() string {
	//Load from config and set server jar filename, if not set in config, default to server.jar
	jarFile, err := cfg.String("paths.jarFile")
	if err != nil {
		jarFile = "server.jar"
	}

	return jarFile
}

func debugIsEnabled() bool {
	debug, _ := cfg.Bool("debug")
	return debug
}
