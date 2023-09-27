// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_getLicense(t *testing.T) {
	type args struct {
		name   string
		values *LicenseValues
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "regular",
			args: args{
				name:   "MIT",
				values: &LicenseValues{Owner: "Gitea", Year: "2023"},
			},
			want: `MIT License

Copyright (c) 2023 Gitea

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
`,
			wantErr: assert.NoError,
		},
		{
			name: "license not found",
			args: args{
				name: "notfound",
			},
			wantErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetLicense(tt.args.name, tt.args.values)
			if !tt.wantErr(t, err, fmt.Sprintf("GetLicense(%v, %v)", tt.args.name, tt.args.values)) {
				return
			}
			assert.Equalf(t, tt.want, string(got), "GetLicense(%v, %v)", tt.args.name, tt.args.values)
		})
	}
}

func Test_fillLicensePlaceholder(t *testing.T) {
	type args struct {
		name   string
		values *LicenseValues
		origin string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "owner",
			args: args{
				name:   "regular",
				values: &LicenseValues{Year: "2023", Owner: "Gitea", Email: "teabot@gitea.io", Repo: "gitea"},
				origin: `
<name of author>
<owner>
[NAME]
[name of copyright owner]
[name of copyright holder]
<COPYRIGHT HOLDERS>
<copyright holders>
<AUTHOR>
<author's name or designee>
[one or more legally recognised persons or entities offering the Work under the terms and conditions of this Licence]
`,
			},
			want: `
Gitea
Gitea
Gitea
Gitea
Gitea
Gitea
Gitea
Gitea
Gitea
Gitea
`,
		},
		{
			name: "email",
			args: args{
				name:   "regular",
				values: &LicenseValues{Year: "2023", Owner: "Gitea", Email: "teabot@gitea.io", Repo: "gitea"},
				origin: `
[EMAIL]
`,
			},
			want: `
teabot@gitea.io
`,
		},
		{
			name: "repo",
			args: args{
				name:   "regular",
				values: &LicenseValues{Year: "2023", Owner: "Gitea", Email: "teabot@gitea.io", Repo: "gitea"},
				origin: `
<program>
<one line to give the program's name and a brief idea of what it does.>
`,
			},
			want: `
gitea
gitea
`,
		},
		{
			name: "year",
			args: args{
				name:   "regular",
				values: &LicenseValues{Year: "2023", Owner: "Gitea", Email: "teabot@gitea.io", Repo: "gitea"},
				origin: `
<year>
[YEAR]
{YEAR}
[yyyy]
[Year]
[year]
`,
			},
			want: `
2023
2023
2023
2023
2023
2023
`,
		},
		{
			name: "0BSD",
			args: args{
				name:   "0BSD",
				values: &LicenseValues{Year: "2023", Owner: "Gitea", Email: "teabot@gitea.io", Repo: "gitea"},
				origin: `
Copyright (C) YEAR by AUTHOR EMAIL

...

... THE AUTHOR BE LIABLE FOR ...
`,
			},
			want: `
Copyright (C) 2023 by Gitea teabot@gitea.io

...

... THE AUTHOR BE LIABLE FOR ...
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, string(fillLicensePlaceholder(tt.args.name, tt.args.values, []byte(tt.args.origin))), "fillLicensePlaceholder(%v, %v, %v)", tt.args.name, tt.args.values, tt.args.origin)
		})
	}
}

func Test_detectLicense(t *testing.T) {
	type DetectLicenseTest struct {
		name string
		arg  string
		want []string
	}

	tests := []DetectLicenseTest{
		{
			name: "empty",
			arg:  "",
			want: nil,
		},
		{
			name: "no detected license",
			arg:  "Copyright (c) 2023 Gitea",
			want: nil,
		},
	}

	LoadRepoConfig()
	err := loadSameLicenses()
	assert.NoError(t, err)
	for _, licenseName := range Licenses {
		license, err := GetLicense(licenseName, &LicenseValues{
			Owner: "Gitea",
			Email: "teabot@gitea.io",
			Repo:  "gitea",
			Year:  time.Now().Format("2006"),
		})
		assert.NoError(t, err)

		tests = append(tests, DetectLicenseTest{
			name: fmt.Sprintf("single license test: %s", licenseName),
			arg:  string(license),
			want: []string{ConvertLicenseName(licenseName)},
		})
	}

	err = initClassifier()
	assert.NoError(t, err)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, detectLicense(tt.arg))
		})
	}

	result := detectLicense(tests[2].arg + tests[3].arg + tests[4].arg)
	t.Run("multiple licenses test", func(t *testing.T) {
		assert.Equal(t, 3, len(result))
		assert.Contains(t, result, tests[2].want[0])
		assert.Contains(t, result, tests[3].want[0])
		assert.Contains(t, result, tests[4].want[0])
	})
}
