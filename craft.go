//
// Copyright (C) 2024 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/craft
//

package main

import (
	"os"
	"strconv"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/jsii-runtime-go"
	"github.com/fogfish/craft/internal/awscraft"
	"github.com/fogfish/tagver"
)

func main() {
	app := awscdk.NewApp(nil)

	// craft-vX
	vsn := FromContextVsn(app)
	config := &awscdk.StackProps{
		Env: &awscdk.Environment{
			Account: jsii.String(os.Getenv("CDK_DEFAULT_ACCOUNT")),
			Region:  jsii.String(os.Getenv("CDK_DEFAULT_REGION")),
		},
	}

	awscraft.New(app,
		&awscraft.CraftProps{
			StackProps:       config,
			Version:          vsn.Get("craft", "main"),
			SourceCodeBucket: FromContext(app, "source-code"),
			Cpu:              FromContextFloat(app, "cpu"),
			Memory:           FromContextFloat(app, "mem"),
			Spot:             FromContextBool(app, "spot"),
		},
	)

	app.Synth(nil)
}

//------------------------------------------------------------------------------

func FromContext(app awscdk.App, key string) string {
	val := app.Node().TryGetContext(jsii.String(key))
	switch v := val.(type) {
	case string:
		return v
	default:
		return ""
	}
}

func FromContextFloat(app awscdk.App, key string) *float64 {
	v := FromContext(app, key)
	if v == "" {
		return nil
	}

	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		panic(err)
	}

	return jsii.Number(f)
}

func FromContextBool(app awscdk.App, key string) *bool {
	switch FromContext(app, key) {
	case "on":
		return jsii.Bool(true)
	case "off":
		return jsii.Bool(false)
	default:
		return nil
	}
}

func FromContextVsn(app awscdk.App) tagver.Versions {
	return tagver.NewVersions(FromContext(app, "vsn"))
}
