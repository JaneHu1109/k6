/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2017 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */
package compiler

import (
	"strings"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
)

func TestTransform(t *testing.T) {
	c := New()
	t.Run("blank", func(t *testing.T) {
		src, _, err := c.Preprocess("", "test.js")
		assert.NoError(t, err)
		assert.Equal(t, `"use strict";`, src)
		// assert.Equal(t, 3, srcmap.Version)
		// assert.Equal(t, "test.js", srcmap.File)
		// assert.Equal(t, "", srcmap.Mappings)
	})
	t.Run("double-arrow", func(t *testing.T) {
		src, _, err := c.Preprocess("()=> true", "test.js")
		assert.NoError(t, err)
		assert.Equal(t, `"use strict";(function () {return true;});`, src)
		// assert.Equal(t, 3, srcmap.Version)
		// assert.Equal(t, "test.js", srcmap.File)
		// assert.Equal(t, "aAAA,qBAAK,IAAL", srcmap.Mappings)
	})
	t.Run("longer", func(t *testing.T) {
		src, _, err := c.Preprocess(strings.Join([]string{
			`function add(a, b) {`,
			`    return a + b;`,
			`};`,
			``,
			`let res = add(1, 2);`,
		}, "\n"), "test.js")
		assert.NoError(t, err)
		assert.Equal(t, strings.Join([]string{
			`"use strict";function add(a, b) {`,
			`    return a + b;`,
			`};`,
			``,
			`var res = add(1, 2);`,
		}, "\n"), src)
		// assert.Equal(t, 3, srcmap.Version)
		// assert.Equal(t, "test.js", srcmap.File)
		// assert.Equal(t, "aAAA,SAASA,GAAT,CAAaC,CAAb,EAAgBC,CAAhB,EAAmB;AACf,WAAOD,IAAIC,CAAX;AACH;;AAED,IAAIC,MAAMH,IAAI,CAAJ,EAAO,CAAP,CAAV", srcmap.Mappings)
	})
}

func TestCompile(t *testing.T) {
	c := New()
	t.Run("ES5", func(t *testing.T) {
		src := `1+(function() { return 2; })()`
		pgm, prePgm, code, err := c.Compile(src, "script.js", "", "", true, CompatibilityModeES51)
		if !assert.NoError(t, err) {
			return
		}
		// Running in ES5 mode, so nothing to preprocess
		assert.Nil(t, prePgm)
		assert.Equal(t, src, code)
		v, err := goja.New().RunProgram(pgm)
		if assert.NoError(t, err) {
			assert.Equal(t, int64(3), v.Export())
		}

		t.Run("Wrap", func(t *testing.T) {
			pgm, prePgm, code, err := c.Compile(src, "script.js",
				"(function(){return ", "})", true, CompatibilityModeES51)
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, `(function(){return 1+(function() { return 2; })()})`, code)

			assert.Nil(t, prePgm)
			v, err = goja.New().RunProgram(pgm)
			if assert.NoError(t, err) {
				fn, ok := goja.AssertFunction(v)
				if assert.True(t, ok, "not a function") {
					v, err := fn(goja.Undefined())
					if assert.NoError(t, err) {
						assert.Equal(t, int64(3), v.Export())
					}
				}
			}
		})

		t.Run("Invalid", func(t *testing.T) {
			src := `1+(function() { return 2; )()`
			_, _, _, err := c.Compile(src, "script.js", "", "", true, CompatibilityModeES6)
			assert.IsType(t, &goja.Exception{}, err)
			assert.Contains(t, err.Error(), `SyntaxError: script.js: Unexpected token (1:26)
> 1 | 1+(function() { return 2; )()`)
		})
	})
	t.Run("ES6", func(t *testing.T) {
		pgm, prePgm, code, err := c.Compile(`1+(()=>2)()`, "script.js", "", "", true, CompatibilityModeES6)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, `"use strict";1 + function () {return 2;}();`, code)
		vm := goja.New()
		_, err = vm.RunProgram(prePgm)
		assert.NoError(t, err)
		v, err := vm.RunProgram(pgm)
		if assert.NoError(t, err) {
			assert.Equal(t, int64(3), v.Export())
		}

		t.Run("Wrap", func(t *testing.T) {
			pgm, prePgm, code, err := c.Compile(`fn(1+(()=>2)())`, "script.js", "(function(fn){", "})", true, CompatibilityModeES6)
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, `(function(fn){"use strict";fn(1 + function () {return 2;}());})`, code)
			rt := goja.New()
			_, err = rt.RunProgram(prePgm)
			assert.NoError(t, err)
			v, err := rt.RunProgram(pgm)
			if assert.NoError(t, err) {
				fn, ok := goja.AssertFunction(v)
				if assert.True(t, ok, "not a function") {
					var out interface{}
					_, err := fn(goja.Undefined(), rt.ToValue(func(v goja.Value) {
						out = v.Export()
					}))
					assert.NoError(t, err)
					assert.Equal(t, int64(3), out)
				}
			}
		})

		t.Run("Invalid", func(t *testing.T) {
			_, _, _, err := c.Compile(`1+(=>2)()`, "script.js", "", "", true, CompatibilityModeES6)
			assert.IsType(t, &goja.Exception{}, err)
			assert.Contains(t, err.Error(), `SyntaxError: script.js: Unexpected token (1:3)
> 1 | 1+(=>2)()`)
		})
	})
}
