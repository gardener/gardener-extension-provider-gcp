// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	_ "embed"
	"text/template"

	"github.com/Masterminds/sprig"
)

var (
	//go:embed templates/main.tpl.tf
	mainFile string
	//go:embed templates/terraform.tfvars
	terraformTFVars []byte
	//go:embed templates/variables.tf
	variablesTF string

	mainTemplate *template.Template
)

func init() {
	var err error
	mainTemplate, err = template.
		New("main.tf").
		Funcs(sprig.TxtFuncMap()).
		Parse(mainFile)

	if err != nil {
		panic(err)
	}
}
