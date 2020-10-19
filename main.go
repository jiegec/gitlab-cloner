package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"

	"github.com/juju/loggo"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

var logger = loggo.GetLogger("gitlab-cloner")

type Project struct {
	Name string `json:"name"`
}

func listProjects(host string, group string, accessToken string) ([]string, error) {
	client := &http.Client{}
	// NOTE: we can use Link header for pagination
	res := []string{}
	for i := 0; true; i++ {
		url := fmt.Sprintf("https://%s/api/v4/groups/%s/projects", host, group)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to create request")
		}

		q := req.URL.Query()
		q.Add("access_token", accessToken)
		q.Add("page", fmt.Sprintf("%d", i))
		q.Add("per_page", "100")
		req.URL.RawQuery = q.Encode()

		resp, err := client.Do(req)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to access gitlab")
		}

		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to read from resp")
		}

		var projects []Project
		err = json.Unmarshal(body, &projects)
		if len(projects) == 0 {
			break
		}

		for _, proj := range projects {
			res = append(res, proj.Name)
		}
	}
	return res, nil
}

func action(c *cli.Context) error {
	loggo.ConfigureLoggers("gitlab-cloner=INFO")
	group := c.String("group")
	host := c.String("host")
	accessToken := c.String("access-token")
	targetDirectory := c.String("target-directory")

	projects, err := listProjects(host, group, accessToken)
	if err != nil {
		logger.Errorf("Failed to list projects: %s", err)
		return nil
	}

	// clone/fetch repos
	os.MkdirAll(targetDirectory, 0755)
	for _, proj := range projects {
		dir := path.Join(targetDirectory, proj)

		if _, err := os.Stat(dir); os.IsNotExist(err) {
			// clone if not exists
			logger.Infof("Cloning %s", proj)
			cmd := exec.Command("git", "clone", fmt.Sprintf("git@%s:%s/%s.git", host, group, proj))
			cmd.Dir = targetDirectory
			err := cmd.Run()
			if err != nil {
				logger.Errorf("Failed to clone %s: %s", proj, err)
				return nil
			}
		} else {
			// fetch if exists
			logger.Infof("Fetching %s", proj)
			cmd := exec.Command("git", "fetch", "origin")
			cmd.Dir = dir
			err := cmd.Run()
			if err != nil {
				logger.Errorf("Failed to fetch %s: %s", proj, err)
				return nil
			}
		}
	}
	return nil
}

func main() {
	app := &cli.App{
		Name:    "gitlab-cloner",
		Version: "1.0",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "group",
				Aliases:  []string{"g"},
				Usage:    "GitLab group name",
				Required: true,
			},
			&cli.StringFlag{
				Name:        "host",
				Aliases:     []string{"H"},
				Usage:       "GitLab host name",
				DefaultText: "gitlab.com",
			},
			&cli.StringFlag{
				Name:     "access-token",
				Aliases:  []string{"t"},
				Usage:    "GitLab access token",
				Required: true,
			},
			&cli.StringFlag{
				Name:        "target-directory",
				Aliases:     []string{"d"},
				Usage:       "Target directory",
				DefaultText: ".",
			},
		},
		Action: action,
	}
	app.Run(os.Args)
}
