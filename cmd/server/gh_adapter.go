package main

import (
	ghpkg "blog/internal/github"
	"blog/internal/post"
)

// ghAdapter 将 github.Client 适配为 post.GHClient 接口（解耦循环依赖）。
type ghAdapter struct{ c *ghpkg.Client }

func (a *ghAdapter) GetRepo(owner, repo string) (*post.GHRepoInfo, error) {
	info, err := a.c.GetRepo(owner, repo)
	if err != nil {
		return nil, err
	}
	return &post.GHRepoInfo{
		Name: info.Name, FullName: info.FullName, Description: info.Description,
		Stars: info.Stars, Forks: info.Forks, Language: info.Language,
		License: info.License, OwnerAvatar: info.OwnerAvatar, OwnerLogin: info.OwnerLogin, HTMLURL: info.HTMLURL,
	}, nil
}

func (a *ghAdapter) GetUser(username string) (*post.GHUserInfo, error) {
	info, err := a.c.GetUser(username)
	if err != nil {
		return nil, err
	}
	return &post.GHUserInfo{
		Login: info.Login, Name: info.Name, Bio: info.Bio, AvatarURL: info.AvatarURL,
		Followers: info.Followers, Following: info.Following, Repos: info.Repos, HTMLURL: info.HTMLURL,
	}, nil
}
