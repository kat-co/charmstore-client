// Copyright 2016 Canonical Ltd.
// Licensed under the GPLv3, see LICENCE file for details.

package charmcmd

import (
	"github.com/juju/cmd"
	"gopkg.in/errgo.v1"
	"gopkg.in/juju/charmrepo.v2-unstable/csclient/params"
	"launchpad.net/gnuflag"
)

type termsCommand struct {
	cmd.CommandBase
	auth string

	username string
	password string
	out      cmd.Output
	user     string
}

// TODO (mattyw) As of 16Mar2016 this is implemented
// in a different way to the description here, but the
// description here shows the intent. The implementation
// will need to be improved when it is supported in the
// terms service.
// The implemenation as of 16Mar2016 simply iterates over
// the charms owned by the user and then gets a list of the
// terms required by these charms. Using this it then produces
// a mapping of term:[]charmUrl to be output to the user.
var termsDoc = `
The terms command lists the terms owned by this user and the
charms that require these terms to b agreed to.

   charm terms
`

// Info implements cmd.Command.Info.
func (c *termsCommand) Info() *cmd.Info {
	return &cmd.Info{
		Name:    "terms",
		Purpose: "lists terms owned by the user",
		Doc:     termsDoc,
	}
}

// SetFlags implements cmd.Command.SetFlags.
func (c *termsCommand) SetFlags(f *gnuflag.FlagSet) {
	c.out.AddFlags(f, "yaml", map[string]cmd.Formatter{
		"yaml": cmd.FormatYaml,
		"json": cmd.FormatJson,
	})
	f.StringVar(&c.user, "u", "", "the given user name")
	addAuthFlag(f, &c.auth)
}

// Init implements cmd.Command.Init.
func (c *termsCommand) Init(args []string) error {
	var err error
	c.username, c.password, err = validateAuthFlag(c.auth)
	if err != nil {
		return errgo.Mask(err)
	}
	return cmd.CheckEmpty(args)
}

type termsResponse struct {
	Terms []string `json:"terms"`
}

// Run implements cmd.Command.Run.
func (c *termsCommand) Run(ctxt *cmd.Context) error {
	client, err := newCharmStoreClient(ctxt, c.username, c.password)
	if err != nil {
		return errgo.Notef(err, "cannot create the charm store client")
	}
	defer client.jar.Save()

	if c.user == "" {
		resp, err := client.WhoAmI()
		if err != nil {
			return errgo.Notef(err, "cannot retrieve identity")
		}
		c.user = resp.User
	}

	if err := validateNames([]string{c.user}); err != nil {
		return errgo.Mask(err)
	}

	// We sort here so that our output to the user will be consistent.
	// TODO (mattyw) This only lists the latest version of each charm
	// which might not be what we want in the future.
	path := "/list?owner=" + c.user + "&sort=name,-series"
	var resp params.ListResponse
	if err := client.Get(path, &resp); err != nil {
		return errgo.Notef(err, "cannot list charms for user %s", path)
	}
	output := make(map[string][]string)
	for _, charm := range resp.Results {
		var resp termsResponse
		// TODO (mattyw) We could make a bulk meta request in future.
		if _, err := client.Meta(charm.Id, &resp); err != nil {
			return errgo.Notef(err, "cannot list terms for charm %s", charm.Id.String())
		}
		for _, term := range resp.Terms {
			output[term] = append(output[term], charm.Id.String())
		}
	}
	return c.out.Write(ctxt, output)
}
