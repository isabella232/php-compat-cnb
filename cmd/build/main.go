package main

import (
	"fmt"
	"os"

	"github.com/cloudfoundry/libcfbuildpack/build"
	"github.com/cloudfoundry/php-compat-cnb/compat"
)

func main() {
	context, err := build.DefaultBuild()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to create a default build context: %s", err)
		os.Exit(101)
	}

	code, err := runBuild(context)
	if err != nil {
		context.Logger.Info(err.Error())
	}

	os.Exit(code)

}

func runBuild(context build.Build) (int, error) {
	context.Logger.Title(context.Buildpack)

	phpCompat, willContribute, err := compat.NewContributor(context)
	if err != nil {
		return context.Failure(102), err
	}

	if willContribute {
		err := phpCompat.Contribute()
		if err != nil {
			return context.Failure(103), err
		}
	}

	return context.Success()
}
