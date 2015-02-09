// Copyright 2015 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package hc

import (
	"errors"
	"testing"

	"gopkg.in/check.v1"
)

func Test(t *testing.T) {
	check.TestingT(t)
}

type HCSuite struct{}

var _ = check.Suite(HCSuite{})

func (HCSuite) TestCheck(c *check.C) {
	AddChecker("success", successChecker)
	AddChecker("failing", failingChecker)
	AddChecker("disabled", disabledChecker)
	expected := []Result{
		{Name: "success", Status: HealthCheckOK},
		{Name: "failing", Status: "fail - something went wrong"},
	}
	result := Check()
	c.Assert(result, check.DeepEquals, expected)
}

func successChecker() error {
	return nil
}

func failingChecker() error {
	return errors.New("something went wrong")
}

func disabledChecker() error {
	return ErrDisabledComponent
}
