package main

import (
	"github.com/codegangsta/cli"
	"github.com/olebedev/config"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
)

func main() {
	app := cli.NewApp()
	app.Name = "msct"
	app.Version = "0.1.0"
	app.Usage = "Minecraft Server Control Tool"
	app.Author = "Nathan Young (http://github.com/nathanpaulyoung)"
	app.Commands = []cli.Command{
		startCommand(),
		resumeCommand(),
	}
	app.Run(os.Args)
}

func startCommand() cli.Command {
	command := cli.Command{
		Name:    "start",
		Aliases: []string{"s"},
		Usage:   "start a server",
		Action: func(c *cli.Context) {
			servername := c.Args().First()
			args := buildInvocation(servername)
			cmd := exec.Command("screen", args...)
			cmd.Dir = buildServerDir(servername)
			if serverExists(servername) {
				if err := cmd.Run(); err != nil {
					os.Exit(999)
				}
			} else {
				println("No server known by the name \"" + servername + "\". Either server.jar is missing or the server directory was not configured before compilation.")
				os.Exit(999)
			}
		},
	}
	return command
}

func resumeCommand() cli.Command {
	command := cli.Command{
		Name:    "resume",
		Aliases: []string{"r"},
		Usage:   "resume a server's screen session",
		Action: func(c *cli.Context) {
			servername := c.Args().First()
			screenname := buildScreenName(servername)
			args := []string{"-x", screenname}
			cmd := exec.Command("screen", args...)
			if serverExists(servername) {
				output, _ := cmd.Output()
				println(output)
			} else {
				println("No server known by the name \"" + servername + "\". Either server.jar is missing or the server directory was not configured before compilation.")
				os.Exit(999)
			}
		},
	}
	return command
}

func serverExists(servername string) bool {
	if _, err := os.Stat(buildServerDir(servername) + getJarFile()); err == nil {
		return true
	}
	return false
}

func loadConfig() *config.Config {
	//Load msct.conf, prefer local over /etc/, and parse yaml
	file := ""
	if _, err := os.Stat("./msct.conf"); err == nil {
		r, _ := ioutil.ReadFile("msct.conf")
		file = string(r)
	} else if _, err := os.Stat("/etc/msct.conf"); err == nil {
		r, _ := ioutil.ReadFile("/etc/msct.conf")
		file = string(r)
	} else {
		println("Cannot locate msct.conf; it should be either alongside the msct binary or in the /etc/ directory.")
		os.Exit(1000)
	}
	cfg, _ := config.ParseYaml(file)

	return cfg
}

func buildScreenName(servername string) string {
	cfg := loadConfig()

	//Load from config and set base screen prefix, if not set in config, default to "msct-"
	screenbasename, err := cfg.String("screenbasename")
	if err != nil {
		screenbasename = "msct-"
	}

	return screenbasename + servername
}

func buildInvocation(servername string) []string {
	cfg := loadConfig()

	//Load from config and set whether to start screen attached or not, if not set in config, default to attached
	startStringAttached, err := cfg.Bool("startStringAttached")
	if err != nil {
		startStringAttached = true
	}
	screenParams := ""
	if startStringAttached == true {
		screenParams = "-dmS"
	} else {
		screenParams = "-mS"
	}

	//Load from config and set java parameters, if not set in config, set reasonable defaults
	ram, err := cfg.String("ram")
	if err != nil {
		ram = "2048M"
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
	args = append(args, screenParams, buildScreenName(servername), "java", "-server", "-Xms"+ram+"M", "-Xmx"+ram+"M")
	args = append(args, javaParamsArray...)
	args = append(args, "-jar", fullpath)

	return args
}

func buildServerDir(servername string) string {
	cfg := loadConfig()

	//Load from config and set msct root directory, if not set in config, default to /opt/minecraft/
	rootdir, err := cfg.String("paths.root")
	if err != nil {
		rootdir = "/opt/minecraft/"
	}

	return rootdir + servername + "/"
}

func getJarFile() string {
	cfg := loadConfig()

	//Load from config and set server jar filename, if not set in config, default to server.jar
	jarfile, err := cfg.String("paths.jarfile")
	if err != nil {
		jarfile = "server.jar"
	}

	return jarfile
}
