// Copyright 2017 The Hugo Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package i18n

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/gohugoio/hugo/modules"

	"github.com/gohugoio/hugo/tpl/tplimpl"

	"github.com/gohugoio/hugo/common/loggers"
	"github.com/gohugoio/hugo/langs"
	"github.com/gohugoio/hugo/resources/page"
	"github.com/spf13/afero"
	"github.com/spf13/viper"

	"github.com/gohugoio/hugo/deps"

	qt "github.com/frankban/quicktest"
	"github.com/gohugoio/hugo/config"
	"github.com/gohugoio/hugo/hugofs"
)

var logger = loggers.NewErrorLogger()

type i18nTest struct {
	name                             string
	data                             map[string][]byte
	args                             interface{}
	lang, id, expected, expectedFlag string
}

var i18nTests = []i18nTest{
	// All translations present
	{
		name: "all-present",
		data: map[string][]byte{
			"en.toml": []byte("[hello]\nother = \"Hello, World!\""),
			"es.toml": []byte("[hello]\nother = \"¡Hola, Mundo!\""),
		},
		args:         nil,
		lang:         "es",
		id:           "hello",
		expected:     "¡Hola, Mundo!",
		expectedFlag: "¡Hola, Mundo!",
	},
	// Translation missing in current language but present in default
	{
		name: "present-in-default",
		data: map[string][]byte{
			"en.toml": []byte("[hello]\nother = \"Hello, World!\""),
			"es.toml": []byte("[goodbye]\nother = \"¡Adiós, Mundo!\""),
		},
		args:         nil,
		lang:         "es",
		id:           "hello",
		expected:     "Hello, World!",
		expectedFlag: "[i18n] hello",
	},
	// Translation missing in default language but present in current
	{
		name: "present-in-current",
		data: map[string][]byte{
			"en.toml": []byte("[goodbye]\nother = \"Goodbye, World!\""),
			"es.toml": []byte("[hello]\nother = \"¡Hola, Mundo!\""),
		},
		args:         nil,
		lang:         "es",
		id:           "hello",
		expected:     "¡Hola, Mundo!",
		expectedFlag: "¡Hola, Mundo!",
	},
	// Translation missing in both default and current language
	{
		name: "missing",
		data: map[string][]byte{
			"en.toml": []byte("[goodbye]\nother = \"Goodbye, World!\""),
			"es.toml": []byte("[goodbye]\nother = \"¡Adiós, Mundo!\""),
		},
		args:         nil,
		lang:         "es",
		id:           "hello",
		expected:     "",
		expectedFlag: "[i18n] hello",
	},
	// Default translation file missing or empty
	{
		name: "file-missing",
		data: map[string][]byte{
			"en.toml": []byte(""),
		},
		args:         nil,
		lang:         "es",
		id:           "hello",
		expected:     "",
		expectedFlag: "[i18n] hello",
	},
	// Context provided
	{
		name: "context-provided",
		data: map[string][]byte{
			"en.toml": []byte("[wordCount]\nother = \"Hello, {{.WordCount}} people!\""),
			"es.toml": []byte("[wordCount]\nother = \"¡Hola, {{.WordCount}} gente!\""),
		},
		args: struct {
			WordCount int
		}{
			50,
		},
		lang:         "es",
		id:           "wordCount",
		expected:     "¡Hola, 50 gente!",
		expectedFlag: "¡Hola, 50 gente!",
	},
	// https://github.com/gohugoio/hugo/issues/7787
	{
		name: "readingTime-one",
		data: map[string][]byte{
			"en.toml": []byte(`[readingTime]
one = "One minute to read"
other = "{{ .Count }} minutes to read"
`),
		},
		args:         1,
		lang:         "en",
		id:           "readingTime",
		expected:     "One minute to read",
		expectedFlag: "One minute to read",
	},
	{
		name: "readingTime-many-dot",
		data: map[string][]byte{
			"en.toml": []byte(`[readingTime]
one = "One minute to read"
other = "{{ . }} minutes to read"
`),
		},
		args:         21,
		lang:         "en",
		id:           "readingTime",
		expected:     "21 minutes to read",
		expectedFlag: "21 minutes to read",
	},
	{
		name: "readingTime-many",
		data: map[string][]byte{
			"en.toml": []byte(`[readingTime]
one = "One minute to read"
other = "{{ .Count }} minutes to read"
`),
		},
		args:         21,
		lang:         "en",
		id:           "readingTime",
		expected:     "21 minutes to read",
		expectedFlag: "21 minutes to read",
	},
	// Issue #8454
	{
		name: "readingTime-map-one",
		data: map[string][]byte{
			"en.toml": []byte(`[readingTime]
one = "One minute to read"
other = "{{ .Count }} minutes to read"
`),
		},
		args:         map[string]interface{}{"Count": 1},
		lang:         "en",
		id:           "readingTime",
		expected:     "One minute to read",
		expectedFlag: "One minute to read",
	},
	{
		name: "readingTime-string-one",
		data: map[string][]byte{
			"en.toml": []byte(`[readingTime]
one = "One minute to read"
other = "{{ . }} minutes to read"
`),
		},
		args:         "1",
		lang:         "en",
		id:           "readingTime",
		expected:     "One minute to read",
		expectedFlag: "One minute to read",
	},
	{
		name: "readingTime-map-many",
		data: map[string][]byte{
			"en.toml": []byte(`[readingTime]
one = "One minute to read"
other = "{{ .Count }} minutes to read"
`),
		},
		args:         map[string]interface{}{"Count": 21},
		lang:         "en",
		id:           "readingTime",
		expected:     "21 minutes to read",
		expectedFlag: "21 minutes to read",
	},
	{
		name: "argument-float",
		data: map[string][]byte{
			"en.toml": []byte(`[float]
other = "Number is {{ . }}"
`),
		},
		args:         22.5,
		lang:         "en",
		id:           "float",
		expected:     "Number is 22.5",
		expectedFlag: "Number is 22.5",
	},
	// Same id and translation in current language
	// https://github.com/gohugoio/hugo/issues/2607
	{
		name: "same-id-and-translation",
		data: map[string][]byte{
			"es.toml": []byte("[hello]\nother = \"hello\""),
			"en.toml": []byte("[hello]\nother = \"hi\""),
		},
		args:         nil,
		lang:         "es",
		id:           "hello",
		expected:     "hello",
		expectedFlag: "hello",
	},
	// Translation missing in current language, but same id and translation in default
	{
		name: "same-id-and-translation-default",
		data: map[string][]byte{
			"es.toml": []byte("[bye]\nother = \"bye\""),
			"en.toml": []byte("[hello]\nother = \"hello\""),
		},
		args:         nil,
		lang:         "es",
		id:           "hello",
		expected:     "hello",
		expectedFlag: "[i18n] hello",
	},
	// Unknown language code should get its plural spec from en
	{
		name: "unknown-language-code",
		data: map[string][]byte{
			"en.toml": []byte(`[readingTime]
one ="one minute read"
other = "{{.Count}} minutes read"`),
			"klingon.toml": []byte(`[readingTime]
one =  "eitt minutt med lesing"
other = "{{ .Count }} minuttar lesing"`),
		},
		args:         3,
		lang:         "klingon",
		id:           "readingTime",
		expected:     "3 minuttar lesing",
		expectedFlag: "3 minuttar lesing",
	},
	// https://github.com/gohugoio/hugo/issues/7798
	{
		name: "known-language-missing-plural",
		data: map[string][]byte{
			"oc.toml": []byte(`[oc]
one =  "abc"`),
		},
		args:         1,
		lang:         "oc",
		id:           "oc",
		expected:     "abc",
		expectedFlag: "abc",
	},
	// https://github.com/gohugoio/hugo/issues/7794
	{
		name: "dotted-bare-key",
		data: map[string][]byte{
			"en.toml": []byte(`"shop_nextPage.one" = "Show Me The Money"

`),
		},
		args:         nil,
		lang:         "en",
		id:           "shop_nextPage.one",
		expected:     "Show Me The Money",
		expectedFlag: "Show Me The Money",
	},
	// https: //github.com/gohugoio/hugo/issues/7804
	{
		name: "lang-with-hyphen",
		data: map[string][]byte{
			"pt-br.toml": []byte(`foo.one =  "abc"`),
		},
		args:         1,
		lang:         "pt-br",
		id:           "foo",
		expected:     "abc",
		expectedFlag: "abc",
	},
}

func doTestI18nTranslate(t testing.TB, test i18nTest, cfg config.Provider) string {
	tp := prepareTranslationProvider(t, test, cfg)
	f := tp.t.Func(test.lang)
	return f(test.id, test.args)
}

type countField struct {
	Count int
}

type noCountField struct {
	Counts int
}

type countMethod struct {
}

func (c countMethod) Count() int {
	return 32
}

func TestGetPluralCount(t *testing.T) {
	c := qt.New(t)

	c.Assert(getPluralCount(map[string]interface{}{"Count": 32}), qt.Equals, 32)
	c.Assert(getPluralCount(map[string]interface{}{"Count": 1}), qt.Equals, 1)
	c.Assert(getPluralCount(map[string]interface{}{"Count": "32"}), qt.Equals, 32)
	c.Assert(getPluralCount(map[string]interface{}{"count": 32}), qt.Equals, 32)
	c.Assert(getPluralCount(map[string]interface{}{"Count": "32"}), qt.Equals, 32)
	c.Assert(getPluralCount(map[string]interface{}{"Counts": 32}), qt.Equals, 0)
	c.Assert(getPluralCount("foo"), qt.Equals, 0)
	c.Assert(getPluralCount(countField{Count: 22}), qt.Equals, 22)
	c.Assert(getPluralCount(&countField{Count: 22}), qt.Equals, 22)
	c.Assert(getPluralCount(noCountField{Counts: 23}), qt.Equals, 0)
	c.Assert(getPluralCount(countMethod{}), qt.Equals, 32)
	c.Assert(getPluralCount(&countMethod{}), qt.Equals, 32)

	c.Assert(getPluralCount(1234), qt.Equals, 1234)
	c.Assert(getPluralCount(1234.4), qt.Equals, 1234)
	c.Assert(getPluralCount(1234.6), qt.Equals, 1234)
	c.Assert(getPluralCount(0.6), qt.Equals, 0)
	c.Assert(getPluralCount(1.0), qt.Equals, 1)
	c.Assert(getPluralCount("1234"), qt.Equals, 1234)
	c.Assert(getPluralCount(nil), qt.Equals, 0)
}

func prepareTranslationProvider(t testing.TB, test i18nTest, cfg config.Provider) *TranslationProvider {
	c := qt.New(t)
	fs := hugofs.NewMem(cfg)

	for file, content := range test.data {
		err := afero.WriteFile(fs.Source, filepath.Join("i18n", file), []byte(content), 0755)
		c.Assert(err, qt.IsNil)
	}

	tp := NewTranslationProvider()
	depsCfg := newDepsConfig(tp, cfg, fs)
	d, err := deps.New(depsCfg)
	c.Assert(err, qt.IsNil)
	c.Assert(d.LoadResources(), qt.IsNil)

	return tp
}

func newDepsConfig(tp *TranslationProvider, cfg config.Provider, fs *hugofs.Fs) deps.DepsCfg {
	l := langs.NewLanguage("en", cfg)
	l.Set("i18nDir", "i18n")
	return deps.DepsCfg{
		Language:            l,
		Site:                page.NewDummyHugoSite(cfg),
		Cfg:                 cfg,
		Fs:                  fs,
		Logger:              logger,
		TemplateProvider:    tplimpl.DefaultTemplateProvider,
		TranslationProvider: tp,
	}
}

func getConfig() *viper.Viper {
	v := viper.New()
	v.SetDefault("defaultContentLanguage", "en")
	v.Set("contentDir", "content")
	v.Set("dataDir", "data")
	v.Set("i18nDir", "i18n")
	v.Set("layoutDir", "layouts")
	v.Set("archetypeDir", "archetypes")
	v.Set("assetDir", "assets")
	v.Set("resourceDir", "resources")
	v.Set("publishDir", "public")
	langs.LoadLanguageSettings(v, nil)
	mod, err := modules.CreateProjectModule(v)
	if err != nil {
		panic(err)
	}
	v.Set("allModules", modules.Modules{mod})

	return v
}

func TestI18nTranslate(t *testing.T) {
	c := qt.New(t)
	var actual, expected string
	v := getConfig()

	// Test without and with placeholders
	for _, enablePlaceholders := range []bool{false, true} {
		v.Set("enableMissingTranslationPlaceholders", enablePlaceholders)

		for _, test := range i18nTests {
			c.Run(fmt.Sprintf("%s-%t", test.name, enablePlaceholders), func(c *qt.C) {
				if enablePlaceholders {
					expected = test.expectedFlag
				} else {
					expected = test.expected
				}
				actual = doTestI18nTranslate(c, test, v)
				c.Assert(actual, qt.Equals, expected)
			})
		}
	}
}

func BenchmarkI18nTranslate(b *testing.B) {
	v := getConfig()
	for _, test := range i18nTests {
		b.Run(test.name, func(b *testing.B) {
			tp := prepareTranslationProvider(b, test, v)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				f := tp.t.Func(test.lang)
				actual := f(test.id, test.args)
				if actual != test.expected {
					b.Fatalf("expected %v got %v", test.expected, actual)
				}
			}
		})
	}
}
