package main

import (
	"fmt"

	"github.com/cloudfoundry-community/go-cfclient"
)

type CfSpaceUserGetter struct {
	cfEnvs map[string]*cfclient.Client
}

func NewCfSpaceUserGetter() *CfSpaceUserGetter {
	return &CfSpaceUserGetter{
		cfEnvs: map[string]*cfclient.Client{},
	}
}

func (su *CfSpaceUserGetter) RegisterEnvironment(name string, client *cfclient.Client) {
	su.cfEnvs[name] = client
}

func (su *CfSpaceUserGetter) Get(env, spaceId string) ([]string, error) {
	cf, ok := su.cfEnvs[env]
	if !ok {
		return nil, fmt.Errorf("Environment %v not configured\n", env)
	}

	space, err := cf.GetSpaceByGuid(spaceId)
	if err != nil {
		return nil, err
	}

	var users []string

	roles, err := space.Roles()
	if err != nil {
		return nil, err
	}

	for _, user := range roles {
		users = append(users, user.Username)
	}

	return users, nil
}
