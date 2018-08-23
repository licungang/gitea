// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth/oauth2"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/urfave/cli"
)

var (
	// CmdAdmin represents the available admin sub-command.
	CmdAdmin = cli.Command{
		Name:  "admin",
		Usage: "Command line interface to perform common administrative operations",
		Subcommands: []cli.Command{
			subcmdCreateUser,
			subcmdChangePassword,
			subcmdRepoSyncReleases,
			subcmdRegenerate,
			subcmdAuth,
		},
	}

	subcmdCreateUser = cli.Command{
		Name:   "create-user",
		Usage:  "Create a new user in database",
		Action: runCreateUser,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "name",
				Usage: "Username",
			},
			cli.StringFlag{
				Name:  "password",
				Usage: "User password",
			},
			cli.StringFlag{
				Name:  "email",
				Usage: "User email address",
			},
			cli.BoolFlag{
				Name:  "admin",
				Usage: "User is an admin",
			},
			cli.StringFlag{
				Name:  "config, c",
				Value: "custom/conf/app.ini",
				Usage: "Custom configuration file path",
			},
		},
	}

	subcmdChangePassword = cli.Command{
		Name:   "change-password",
		Usage:  "Change a user's password",
		Action: runChangePassword,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "username,u",
				Value: "",
				Usage: "The user to change password for",
			},
			cli.StringFlag{
				Name:  "password,p",
				Value: "",
				Usage: "New password to set for user",
			},
			cli.StringFlag{
				Name:  "config, c",
				Value: "custom/conf/app.ini",
				Usage: "Custom configuration file path",
			},
		},
	}

	subcmdRepoSyncReleases = cli.Command{
		Name:   "repo-sync-releases",
		Usage:  "Synchronize repository releases with tags",
		Action: runRepoSyncReleases,
	}

	subcmdRegenerate = cli.Command{
		Name:  "regenerate",
		Usage: "Regenerate specific files",
		Subcommands: []cli.Command{
			microcmdRegenHooks,
			microcmdRegenKeys,
		},
	}

	microcmdRegenHooks = cli.Command{
		Name:   "hooks",
		Usage:  "Regenerate git-hooks",
		Action: runRegenerateHooks,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "config, c",
				Value: "custom/conf/app.ini",
				Usage: "Custom configuration file path",
			},
		},
	}

	microcmdRegenKeys = cli.Command{
		Name:   "keys",
		Usage:  "Regenerate authorized_keys file",
		Action: runRegenerateKeys,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "config, c",
				Value: "custom/conf/app.ini",
				Usage: "Custom configuration file path",
			},
		},
	}

	subcmdAuth = cli.Command{
		Name:  "auth",
		Usage: "Modify external auth providers",
		Subcommands: []cli.Command{
			microcmdAuthAddOauth,
		},
	}

	microcmdAuthList = cli.Command{
		Name:   "list",
		Usage:  "List auth sources",
		Action: runListAuth,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "config, c",
				Value: "custom/conf/app.ini",
				Usage: "Custom configuration file path",
			},
		},
	}

	microcmdAuthUpdateOauth = cli.Command{
		Name:   "update-oauth",
		Usage:  "Update existing Oauth authentication source",
		Action: runUpdateOauth,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "config, c",
				Value: "custom/conf/app.ini",
				Usage: "Custom configuration file path",
			},
			cli.Int64Flag{
				Name:  "id",
				Usage: "ID of Oauth authentication source that will be updated",
			},
			cli.StringFlag{
				Name:  "name",
				Value: "",
				Usage: "Application Name",
			},
			cli.StringFlag{
				Name:  "provider",
				Value: "",
				Usage: "OAuth2 Provider",
			},
			cli.StringFlag{
				Name:  "key",
				Value: "",
				Usage: "Client ID (Key)",
			},
			cli.StringFlag{
				Name:  "secret",
				Value: "",
				Usage: "Client Secret",
			},
			cli.StringFlag{
				Name:  "secret",
				Value: "",
				Usage: "Client Secret",
			},
			cli.StringFlag{
				Name:  "auto-discover-url",
				Value: "",
				Usage: "OpenID Connect Auto Discovery URL (only required when using OpenID Connect as provider)",
			},
			cli.StringFlag{
				Name:  "use-custom-urls",
				Value: "false",
				Usage: "Use custom URLs for GitLab/GitHub OAuth endpoints",
			},
			cli.StringFlag{
				Name:  "custom-auth-url",
				Value: "",
				Usage: "Use a custom Authorization URL (option for GitLab/GitHub)",
			},
			cli.StringFlag{
				Name:  "custom-token-url",
				Value: "",
				Usage: "Use a custom Token URL (option for GitLab/GitHub)",
			},
			cli.StringFlag{
				Name:  "custom-profile-url",
				Value: "",
				Usage: "Use a custom Profile URL (option for GitLab/GitHub)",
			},
			cli.StringFlag{
				Name:  "custom-email-url",
				Value: "",
				Usage: "Use a custom Email URL (option for GitHub)",
			},
		},
	}

	microcmdAuthAddOauth = cli.Command{
		Name:   "add-oauth",
		Usage:  "Add new Oauth authentication source",
		Action: runAddOauth,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "config, c",
				Value: "custom/conf/app.ini",
				Usage: "Custom configuration file path",
			},
			cli.StringFlag{
				Name:  "name",
				Value: "",
				Usage: "Application Name",
			},
			cli.StringFlag{
				Name:  "provider",
				Value: "",
				Usage: "OAuth2 Provider",
			},
			cli.StringFlag{
				Name:  "key",
				Value: "",
				Usage: "Client ID (Key)",
			},
			cli.StringFlag{
				Name:  "secret",
				Value: "",
				Usage: "Client Secret",
			},
			cli.StringFlag{
				Name:  "secret",
				Value: "",
				Usage: "Client Secret",
			},
			cli.StringFlag{
				Name:  "auto-discover-url",
				Value: "",
				Usage: "OpenID Connect Auto Discovery URL (only required when using OpenID Connect as provider)",
			},
			cli.StringFlag{
				Name:  "use-custom-urls",
				Value: "false",
				Usage: "Use custom URLs for GitLab/GitHub OAuth endpoints",
			},
			cli.StringFlag{
				Name:  "custom-auth-url",
				Value: "",
				Usage: "Use a custom Authorization URL (option for GitLab/GitHub)",
			},
			cli.StringFlag{
				Name:  "custom-token-url",
				Value: "",
				Usage: "Use a custom Token URL (option for GitLab/GitHub)",
			},
			cli.StringFlag{
				Name:  "custom-profile-url",
				Value: "",
				Usage: "Use a custom Profile URL (option for GitLab/GitHub)",
			},
			cli.StringFlag{
				Name:  "custom-email-url",
				Value: "",
				Usage: "Use a custom Email URL (option for GitHub)",
			},
		},
	}
)

func runChangePassword(c *cli.Context) error {
	if err := argsSet(c, "username", "password"); err != nil {
		return err
	}

	if c.IsSet("config") {
		setting.CustomConf = c.String("config")
	}

	if err := initDB(); err != nil {
		return err
	}

	uname := c.String("username")
	user, err := models.GetUserByName(uname)
	if err != nil {
		return err
	}
	if user.Salt, err = models.GetUserSalt(); err != nil {
		return err
	}
	user.HashPassword(c.String("password"))
	if err := models.UpdateUserCols(user, "passwd", "salt"); err != nil {
		return err
	}

	fmt.Printf("%s's password has been successfully updated!\n", user.Name)
	return nil
}

func runCreateUser(c *cli.Context) error {
	if err := argsSet(c, "name", "password", "email"); err != nil {
		return err
	}

	if c.IsSet("config") {
		setting.CustomConf = c.String("config")
	}

	if err := initDB(); err != nil {
		return err
	}

	if err := models.CreateUser(&models.User{
		Name:     c.String("name"),
		Email:    c.String("email"),
		Passwd:   c.String("password"),
		IsActive: true,
		IsAdmin:  c.Bool("admin"),
	}); err != nil {
		return fmt.Errorf("CreateUser: %v", err)
	}

	fmt.Printf("New user '%s' has been successfully created!\n", c.String("name"))
	return nil
}

func runRepoSyncReleases(c *cli.Context) error {
	if err := initDB(); err != nil {
		return err
	}

	log.Trace("Synchronizing repository releases (this may take a while)")
	for page := 1; ; page++ {
		repos, count, err := models.SearchRepositoryByName(&models.SearchRepoOptions{
			Page:     page,
			PageSize: models.RepositoryListDefaultPageSize,
			Private:  true,
		})
		if err != nil {
			return fmt.Errorf("SearchRepositoryByName: %v", err)
		}
		if len(repos) == 0 {
			break
		}
		log.Trace("Processing next %d repos of %d", len(repos), count)
		for _, repo := range repos {
			log.Trace("Synchronizing repo %s with path %s", repo.FullName(), repo.RepoPath())
			gitRepo, err := git.OpenRepository(repo.RepoPath())
			if err != nil {
				log.Warn("OpenRepository: %v", err)
				continue
			}

			oldnum, err := getReleaseCount(repo.ID)
			if err != nil {
				log.Warn(" GetReleaseCountByRepoID: %v", err)
			}
			log.Trace(" currentNumReleases is %d, running SyncReleasesWithTags", oldnum)

			if err = models.SyncReleasesWithTags(repo, gitRepo); err != nil {
				log.Warn(" SyncReleasesWithTags: %v", err)
				continue
			}

			count, err = getReleaseCount(repo.ID)
			if err != nil {
				log.Warn(" GetReleaseCountByRepoID: %v", err)
				continue
			}

			log.Trace(" repo %s releases synchronized to tags: from %d to %d",
				repo.FullName(), oldnum, count)
		}
	}

	return nil
}

func getReleaseCount(id int64) (int64, error) {
	return models.GetReleaseCountByRepoID(
		id,
		models.FindReleasesOptions{
			IncludeTags: true,
		},
	)
}

func runRegenerateHooks(c *cli.Context) error {
	if c.IsSet("config") {
		setting.CustomConf = c.String("config")
	}

	if err := initDB(); err != nil {
		return err
	}
	return models.SyncRepositoryHooks()
}

func runRegenerateKeys(c *cli.Context) error {
	if c.IsSet("config") {
		setting.CustomConf = c.String("config")
	}

	if err := initDB(); err != nil {
		return err
	}
	return models.RewriteAllPublicKeys()
}

func parseOAuth2Config(c *cli.Context) *models.OAuth2Config {
	var customURLMapping *oauth2.CustomURLMapping
	if c.IsSet("use-custom-urls") {
		customURLMapping = &oauth2.CustomURLMapping{
			TokenURL:   c.String("custom-token-url"),
			AuthURL:    c.String("custom-auth-url"),
			ProfileURL: c.String("custom-profile-url"),
			EmailURL:   c.String("custom-email-url"),
		}
	} else {
		customURLMapping = nil
	}
	return &models.OAuth2Config{
		Provider:                      c.String("provider"),
		ClientID:                      c.String("key"),
		ClientSecret:                  c.String("secret"),
		OpenIDConnectAutoDiscoveryURL: c.String("auto-discover-url"),
		CustomURLMapping:              customURLMapping,
	}
}

func runAddOauth(c *cli.Context) error {
	if c.IsSet("config") {
		setting.CustomConf = c.String("config")
	}

	if err := initDB(); err != nil {
		return err
	}

	if err := models.CreateLoginSource(&models.LoginSource{
		Type:      models.LoginOAuth2,
		Name:      c.String("name"),
		IsActived: true,
		Cfg:       parseOAuth2Config(c),
	}); err != nil {
		return err
	}

	return nil
}

func runUpdateOauth(c *cli.Context) error {
	if c.IsSet("config") {
		setting.CustomConf = c.String("config")
	}

	if !c.IsSet("id") {
		return fmt.Errorf("--id flag is missing")
	}

	if err := initDB(); err != nil {
		return err
	}

	if err := models.UpdateSource(&models.LoginSource{
		ID:        c.Int64("id"),
		Type:      models.LoginOAuth2,
		Name:      c.String("name"),
		IsActived: true,
		Cfg:       parseOAuth2Config(c),
	}); err != nil {
		return err
	}

	return nil
}

func runListAuth(c *cli.Context) error {
	if c.IsSet("config") {
		setting.CustomConf = c.String("config")
	}

	if err := initDB(); err != nil {
		return err
	}

	loginSources, err := models.LoginSources()

	if err != nil {
		return err
	}

	// loop through each source and print
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.AlignRight)
	fmt.Fprintf(w, "ID\tName\tType\tEnabled")
	for _, source := range loginSources {
		fmt.Fprintf(w, "%d\t%s\t%s\t%t", source.ID, source.Name, models.LoginNames[source.Type], source.IsActived)
	}
	w.Flush()

	return nil
}
