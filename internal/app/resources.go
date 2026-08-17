package app

import (
	"fmt"
	"path/filepath"

	ghpkg "blog/internal/github"
	"blog/internal/post"
	tmpl "blog/internal/settings"
)

func loadIcons(root string, devMode bool, requested string) (*tmpl.IconRegistry, error) {
	path := requested
	if devMode {
		if path == "" {
			path = filepath.Join(root, "internal", "settings", "data", "icons.yml")
		}
		return tmpl.LoadIcons(path)
	}
	return tmpl.LoadIconsEmbed()
}

// githubPostAdapter keeps the post package independent from the GitHub HTTP
// client while allowing the application owner to wire both shared services.
type githubPostAdapter struct{ client *ghpkg.Client }

func (a githubPostAdapter) GetRepo(owner, repo string) (*post.GHRepoInfo, error) {
	if a.client == nil {
		return nil, fmt.Errorf("github client is not configured")
	}
	info, err := a.client.GetRepo(owner, repo)
	if err != nil {
		return nil, err
	}
	return &post.GHRepoInfo{
		Name: info.Name, FullName: info.FullName, Description: info.Description,
		Stars: info.Stars, Forks: info.Forks, Language: info.Language,
		License: info.License, OwnerAvatar: info.OwnerAvatar, OwnerLogin: info.OwnerLogin,
		HTMLURL: info.HTMLURL,
	}, nil
}

func (a githubPostAdapter) GetUser(username string) (*post.GHUserInfo, error) {
	if a.client == nil {
		return nil, fmt.Errorf("github client is not configured")
	}
	info, err := a.client.GetUser(username)
	if err != nil {
		return nil, err
	}
	return &post.GHUserInfo{
		Login: info.Login, Name: info.Name, Bio: info.Bio, AvatarURL: info.AvatarURL,
		Followers: info.Followers, Following: info.Following, Repos: info.Repos, HTMLURL: info.HTMLURL,
	}, nil
}

var _ post.GHClient = githubPostAdapter{}
