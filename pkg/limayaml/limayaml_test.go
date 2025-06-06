// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limayaml

import (
	"encoding/json"
	"os"
	"testing"

	"gotest.tools/v3/assert"
)

func dumpJSON(t *testing.T, d any) string {
	b, err := json.Marshal(d)
	assert.NilError(t, err)
	return string(b)
}

const emptyYAML = "{}\n"

func TestEmptyYAML(t *testing.T) {
	var y LimaYAML
	t.Log(dumpJSON(t, y))
	b, err := Marshal(&y, false)
	assert.NilError(t, err)
	assert.Equal(t, string(b), emptyYAML)
}

const defaultYAML = "{}\n"

func TestDefaultYAML(t *testing.T) {
	bytes, err := os.ReadFile("default.yaml")
	assert.NilError(t, err)
	var y LimaYAML
	err = Unmarshal(bytes, &y, "")
	assert.NilError(t, err)
	y.Images = nil                // remove default images
	y.Mounts = nil                // remove default mounts
	y.Base = nil                  // remove default base templates
	y.MinimumLimaVersion = nil    // remove minimum Lima version
	y.MountTypesUnsupported = nil // remove default workaround for kernel 6.9-6.11
	t.Log(dumpJSON(t, y))
	b, err := Marshal(&y, false)
	assert.NilError(t, err)
	assert.Equal(t, string(b), defaultYAML)
}
