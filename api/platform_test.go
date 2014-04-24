// Copyright 2014 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
    "launchpad.net/gocheck"
	"net/http"
	"net/http/httptest"
    "fmt"
    "strings"
)


type PlatformSuite struct {}

var _ = gocheck.Suite(&PlatformSuite{})

func (p *PlatformSuite) TestPlatformAdd(c *gocheck.C) {
    dockerfile_url := "http://localhost/Dockerfile"

    body := fmt.Sprintf("name=%s&dockerfile=%s", "teste", dockerfile_url)
	request, _ := http.NewRequest("PUT", "/platform/add", strings.NewReader(body))
    recorder := httptest.NewRecorder()

    result := platformAdd(recorder, request, nil)

    c.Assert(result, gocheck.IsNil)
}
