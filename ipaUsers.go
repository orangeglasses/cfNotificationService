package main

import (
	"fmt"

	"github.com/vchrisr/go-freeipa/freeipa"
)

type IpaUserGetter struct {
	ipaClient *freeipa.Client
}

func NewIpaUserGetter(client *freeipa.Client) *IpaUserGetter {

	return &IpaUserGetter{
		ipaClient: client,
	}
}

func (iu *IpaUserGetter) Get(env, group string) ([]string, error) { //env is ignored
	result, err := iu.ipaClient.GroupShow(&freeipa.GroupShowArgs{
		Cn: group,
	}, &freeipa.GroupShowOptionalArgs{})
	if err != nil {
		return nil, err
	}

	users := *result.Result.MemberUser
	fmt.Println(users)

	return users, nil
}
