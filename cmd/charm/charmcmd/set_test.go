// Copyright 2015 Canonical Ltd.
// Licensed under the GPLv3, see LICENCE file for details.

package charmcmd_test

import (
	"encoding/json"
	"fmt"

	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"
	"gopkg.in/juju/charm.v6-unstable"
	"gopkg.in/juju/charmrepo.v2-unstable/csclient/params"
	"gopkg.in/macaroon-bakery.v1/bakery/checkers"

	"github.com/juju/charmstore-client/internal/entitytesting"
)

type setSuite struct {
	commonSuite
}

var _ = gc.Suite(&setSuite{})

func (s *setSuite) SetUpTest(c *gc.C) {
	s.commonSuite.SetUpTest(c)
	s.discharge = func(cavId, cav string) ([]checkers.Caveat, error) {
		return []checkers.Caveat{
			checkers.DeclaredCaveat("username", "charmers"),
		}, nil
	}
}

var setInitErrorTests = []struct {
	args []string
	err  string
}{{
	err: "no charm or bundle id specified",
}, {
	args: []string{"homepage=value"},
	err:  `invalid charm or bundle id: URL has invalid charm or bundle name: "homepage=value"`,
}, {
	args: []string{"invalid:entity", "homepage=value"},
	err:  `invalid charm or bundle id: charm or bundle URL has invalid schema: "invalid:entity"`,
}, {
	args: []string{"wordpress"},
	err:  "no set arguments provided",
}, {
	args: []string{"wordpress", "homepage"},
	err:  `invalid set arguments: expected "key=value" or "key:=value", got "homepage"`,
}, {
	args: []string{"wordpress", "homepage=value1", "=value2", "bugs-url=value3"},
	err:  `invalid set arguments: expected "key=value" or "key:=value", got "=value2"`,
}, {
	args: []string{"wordpress", "homepage=value1", "bugs-url:42"},
	err:  `invalid set arguments: expected "key=value" or "key:=value", got "bugs-url:42"`,
}, {
	args: []string{"wordpress", "homepage:=value1"},
	err:  "invalid set arguments: invalid JSON in key homepage: invalid character 'v' looking for beginning of value",
}, {
	args: []string{"wordpress", "homepage:="},
	err:  "invalid set arguments: invalid JSON in key homepage: unexpected end of JSON input",
}, {
	args: []string{"wordpress", "homepage=value1", "bugs-url=value2", "homepage:=42"},
	err:  `invalid set arguments: key "homepage" specified more than once`,
}, {
	args: []string{"wordpress", "name=value", "--auth", "bad-wolf"},
	err:  `invalid auth credentials: expected "user:passwd", got "bad-wolf"`,
}}

func (s *setSuite) TestInitError(c *gc.C) {
	dir := c.MkDir()
	for i, test := range setInitErrorTests {
		c.Logf("test %d: %q", i, test.args)
		args := []string{"set"}
		stdout, stderr, code := run(dir, append(args, test.args...)...)
		c.Assert(stdout, gc.Equals, "")
		c.Assert(stderr, gc.Matches, "error: "+test.err+"\n")
		c.Assert(code, gc.Equals, 2)
	}
}

func (s *setSuite) TestRunError(c *gc.C) {
	stdout, stderr, code := run(c.MkDir(), "set", "no-such-entity", "homepage=value")
	c.Assert(stdout, gc.Equals, "")
	c.Assert(stderr, gc.Matches, "ERROR cannot update the set arguments provided: no matching charm or bundle for cs:no-such-entity\n")
	c.Assert(code, gc.Equals, 1)
}

func (s *setSuite) TestAuthenticationError(c *gc.C) {
	s.discharge = func(cond, arg string) ([]checkers.Caveat, error) {
		return nil, fmt.Errorf("no discharge")
	}
	url := charm.MustParseURL("~charmers/utopic/wordpress-42")
	s.uploadCharmDir(c, url, -1, entitytesting.Repo.CharmDir("wordpress"))
	stdout, stderr, code := run(c.MkDir(), "set", url.String(), "homepage=value")
	c.Assert(stdout, gc.Equals, "")
	c.Assert(stderr, gc.Matches, "ERROR cannot update the set arguments provided: cannot get discharge from \".*\": third party refused discharge: cannot discharge: no discharge\n")
	c.Assert(code, gc.Equals, 1)
}

var setCommonSuccessTests = []struct {
	about            string
	args             []string
	initialCommon    map[string]interface{}
	initialExtra     map[string]interface{}
	expectCommonInfo map[string]interface{}
	expectExtraInfo  map[string]interface{}
}{{
	about: "single update",
	args:  []string{"homepage=value"},
	expectCommonInfo: map[string]interface{}{
		"homepage": "value",
	},
	expectExtraInfo: map[string]interface{}{},
}, {
	about: "string fields",
	args:  []string{"homepage=value1", "bugs-url=2"},
	expectCommonInfo: map[string]interface{}{
		"homepage": "value1",
		"bugs-url": "2",
	},
	expectExtraInfo: map[string]interface{}{},
}, {
	about: "empty update",
	args:  []string{"homepage=value1", "bugs-url="},
	expectCommonInfo: map[string]interface{}{
		"homepage": "value1",
		"bugs-url": "",
	},
	expectExtraInfo: map[string]interface{}{},
}, {
	about: "overriding existing values",
	args:  []string{"homepage=http://myhomepage.com", "bugs-url="},
	initialCommon: map[string]interface{}{
		"homepage": "http://myoldhomepage.com",
		"bugs-url": "http://myhomepage.com/bugs",
	},
	expectCommonInfo: map[string]interface{}{
		"homepage": "http://myhomepage.com",
		"bugs-url": "",
	},
	expectExtraInfo: map[string]interface{}{},
}, {
	about: "extending existing common-info",
	args:  []string{`bugs-url=some new value`},
	initialCommon: map[string]interface{}{
		"homepage": "value1",
	},
	expectCommonInfo: map[string]interface{}{
		"homepage": "value1",
		"bugs-url": "some new value",
	},
	expectExtraInfo: map[string]interface{}{},
}, {
	about: "single update",
	args:  []string{"name=value"},
	expectExtraInfo: map[string]interface{}{
		"name": "value",
	},
	expectCommonInfo: map[string]interface{}{},
}, {
	about: "string fields",
	args:  []string{"name1=value1", "name2=2", "name3=false"},
	expectExtraInfo: map[string]interface{}{
		"name1": "value1",
		"name2": "2",
		"name3": "false",
	},
	expectCommonInfo: map[string]interface{}{},
}, {
	about: "JSON data fields",
	args:  []string{"bool1:=true", "num1:=42", "num2:=47", "slice1:=[true, false]", "bool2:=false", "slice2:=[3, 14]"},
	expectExtraInfo: map[string]interface{}{
		"bool1":  true,
		"num1":   42,
		"num2":   47,
		"slice1": []bool{true, false},
		"bool2":  false,
		"slice2": []int{3, 14},
	},
	expectCommonInfo: map[string]interface{}{},
}, {
	about: "empty update",
	args:  []string{"name1=value1", "name2=", "name3="},
	expectExtraInfo: map[string]interface{}{
		"name1": "value1",
		"name2": "",
		"name3": "",
	},
	expectCommonInfo: map[string]interface{}{},
}, {
	about: "overriding existing values",
	args:  []string{"name1:=42", "name2=", "name4=yes"},
	initialExtra: map[string]interface{}{
		"name1": "value1",
		"name2": 2,
		"name3": false,
	},
	expectExtraInfo: map[string]interface{}{
		"name1": 42,
		"name2": "",
		"name3": false,
		"name4": "yes",
	},
	expectCommonInfo: map[string]interface{}{},
}, {
	about: "extending existing extra-info",
	args:  []string{`newKey=some new value`},
	initialExtra: map[string]interface{}{
		"name1": "value1",
		"name2": 2,
		"name3": false,
	},
	expectExtraInfo: map[string]interface{}{
		"name1":  "value1",
		"name2":  2,
		"name3":  false,
		"newKey": "some new value",
	},
	expectCommonInfo: map[string]interface{}{},
}, {
	about: "Mix extra and common",
	args:  []string{`name=value1`, `bugs-url=value2`},
	expectExtraInfo: map[string]interface{}{
		"name": "value1",
	},
	expectCommonInfo: map[string]interface{}{
		"bugs-url": "value2",
	},
}}

func (s *setSuite) TestSuccess(c *gc.C) {
	for i, test := range setCommonSuccessTests {
		ch := entitytesting.Repo.CharmDir("wordpress")
		url := charm.MustParseURL(fmt.Sprint("~charmers/utopic/wordpress", i))
		dir := c.MkDir()
		c.Logf("test %d: %s", i, test.about)
		url.Revision = i
		s.uploadCharmDir(c, url, -1, ch)
		s.publish(c, url, params.StableChannel)

		// Set initial common-info and extra-info on the charm if required.
		if test.initialCommon != nil {
			s.setCommon(c, url, test.initialCommon)
		}
		if test.initialExtra != nil {
			s.setExtra(c, url, test.initialExtra)
		}

		var msg json.RawMessage
		err := s.client.Get("/"+url.Path()+"/meta/common-info", &msg)
		c.Assert(err, gc.IsNil)
		c.Logf("%s ## %s", string(msg))

		// Check that the command succeeded.
		args := []string{"set", url.Path()}
		stdout, stderr, code := run(dir, append(args, test.args...)...)
		c.Assert(stdout, gc.Equals, "")
		c.Assert(stderr, gc.Matches, "")
		c.Assert(code, gc.Equals, 0)

		// Check that the entity has been updated.
		expect := map[string]interface{}{
			"Id": url,
			"Meta": map[string]interface{}{
				"extra-info":  test.expectExtraInfo,
				"common-info": test.expectCommonInfo,
			},
		}
		c.Assert(s.getInfo(c, url), jc.JSONEquals, expect)
	}
}

func (s *setSuite) TestSuccessfulWithChannel(c *gc.C) {
	ch := entitytesting.Repo.CharmDir("wordpress")
	url := charm.MustParseURL("~charmers/utopic/wordpress")
	s.uploadCharmDir(c, url.WithRevision(40), -1, ch)
	s.uploadCharmDir(c, url.WithRevision(41), -1, ch)
	s.uploadCharmDir(c, url.WithRevision(42), -1, ch)
	s.uploadCharmDir(c, url.WithRevision(43), -1, ch)
	s.publish(c, url.WithRevision(41), params.StableChannel)

	s.publish(c, url.WithRevision(42), params.DevelopmentChannel)

	dir := c.MkDir()

	expectStable := map[string]interface{}{
		"Id": url.WithRevision(41),
		"Meta": map[string]interface{}{
			"extra-info":  map[string]interface{}{},
			"common-info": map[string]interface{}{},
		},
	}
	expectDevelopment := map[string]interface{}{
		"Id": url.WithRevision(42),
		"Meta": map[string]interface{}{
			"extra-info": map[string]interface{}{
				"name": "value",
			},
			"common-info": map[string]interface{}{},
		},
	}
	// Test with the development channel
	_, stderr, code := run(dir, "set", url.String(), "name=value", "-c", "development")
	c.Assert(stderr, gc.Equals, "")
	c.Assert(code, gc.Equals, 0)
	c.Assert(s.getInfo(c, url.WithRevision(42)), jc.JSONEquals, expectDevelopment)
	c.Assert(s.getInfo(c, url.WithRevision(41)), jc.JSONEquals, expectStable)

	// Test with the stable channel
	_, stderr, code = run(dir, "set", url.String(), "name=value1")
	c.Assert(stderr, gc.Equals, "")
	c.Assert(code, gc.Equals, 0)
	expectStable["Meta"].(map[string]interface{})["extra-info"] = map[string]interface{}{
		"name": "value1",
	}
	c.Assert(s.getInfo(c, url.WithRevision(41)), jc.JSONEquals, expectStable)
	c.Assert(s.getInfo(c, url.WithRevision(42)), jc.JSONEquals, expectDevelopment)
}

// getInfo returns the common and extra info for the given entity id as a JSON
// encoded string.
func (s *setSuite) getInfo(c *gc.C, id *charm.URL) string {
	var msg json.RawMessage
	err := s.client.Get("/"+id.Path()+"/meta/any?include=common-info&include=extra-info", &msg)
	c.Assert(err, gc.IsNil)
	return string(msg)
}

// setCommon sets the common info for the given entity id.
func (s *setSuite) setCommon(c *gc.C, id *charm.URL, common map[string]interface{}) {
	err := s.client.PutCommonInfo(id, common)
	c.Assert(err, gc.IsNil)
}

// setExtra sets the extra info for the given entity id.
func (s *setSuite) setExtra(c *gc.C, id *charm.URL, extra map[string]interface{}) {
	err := s.client.PutExtraInfo(id, extra)
	c.Assert(err, gc.IsNil)
}
