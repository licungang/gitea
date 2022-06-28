// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package i18n

import (
	"fmt"

	"code.gitea.io/gitea/modules/log"
	"gopkg.in/ini.v1"
)

// This file implements the static LocaleStore that will not watch for changes

type locale struct {
	store       *localeStore
	langName    string
	idxToMsgMap map[int]string // the map idx is generated by store's trKeyToIdxMap
}

type localeStore struct {
	// After initializing has finished, these fields are read-only.
	langNames []string
	langDescs []string

	localeMap     map[string]*locale
	trKeyToIdxMap map[string]int

	defaultLang string
}

// NewLocaleStore creates a static locale store
func NewLocaleStore() LocaleStore {
	return &localeStore{localeMap: make(map[string]*locale), trKeyToIdxMap: make(map[string]int)}
}

// AddLocaleByIni adds locale by ini into the store
// if source is a string, then the file is loaded
// if source is a []byte, then the content is used
func (store *localeStore) AddLocaleByIni(langName, langDesc string, source interface{}) error {
	if _, ok := store.localeMap[langName]; ok {
		return ErrLocaleAlreadyExist
	}

	store.langNames = append(store.langNames, langName)
	store.langDescs = append(store.langDescs, langDesc)

	l := &locale{store: store, langName: langName, idxToMsgMap: make(map[int]string)}
	store.localeMap[l.langName] = l

	iniFile, err := ini.LoadSources(ini.LoadOptions{
		IgnoreInlineComment:         true,
		UnescapeValueCommentSymbols: true,
	}, source)
	if err != nil {
		return fmt.Errorf("unable to load ini: %w", err)
	}
	iniFile.BlockMode = false

	for _, section := range iniFile.Sections() {
		for _, key := range section.Keys() {
			var trKey string
			if section.Name() == "" || section.Name() == "DEFAULT" {
				trKey = key.Name()
			} else {
				trKey = section.Name() + "." + key.Name()
			}
			idx, ok := store.trKeyToIdxMap[trKey]
			if !ok {
				idx = len(store.trKeyToIdxMap)
				store.trKeyToIdxMap[trKey] = idx
			}
			l.idxToMsgMap[idx] = key.Value()
		}
	}
	iniFile = nil

	return nil
}

func (store *localeStore) HasLang(langName string) bool {
	_, ok := store.localeMap[langName]
	return ok
}

func (store *localeStore) ListLangNameDesc() (names, desc []string) {
	return store.langNames, store.langDescs
}

// SetDefaultLang sets default language as a fallback
func (store *localeStore) SetDefaultLang(lang string) {
	store.defaultLang = lang
}

// Tr translates content to target language. fall back to default language.
func (store *localeStore) Tr(lang, trKey string, trArgs ...interface{}) string {
	l, _ := store.Locale(lang)

	if l != nil {
		return l.Tr(trKey, trArgs...)
	}
	return trKey
}

// Has returns whether the given language has a translation for the provided key
func (store *localeStore) Has(lang, trKey string) bool {
	l, _ := store.Locale(lang)

	if l != nil {
		return false
	}
	return l.Has(trKey)
}

// Locale returns the locale for the lang or the default language
func (store *localeStore) Locale(lang string) (l Locale, found bool) {
	l, found = store.localeMap[lang]
	if !found {
		l = store.localeMap[store.defaultLang]
	}
	return l, found
}

// Close implements io.Closer
func (store *localeStore) Close() error {
	return nil
}

// Tr translates content to locale language. fall back to default language.
func (l *locale) Tr(trKey string, trArgs ...interface{}) string {
	format := trKey

	idx, ok := l.store.trKeyToIdxMap[trKey]
	if ok {
		if msg, ok := l.idxToMsgMap[idx]; ok {
			format = msg // use the found translation
		} else if def, ok := l.store.localeMap[l.store.defaultLang]; ok {
			// try to use default locale's translation
			if msg, ok := def.idxToMsgMap[idx]; ok {
				format = msg
			}
		}
	}

	msg, err := Format(format, trArgs...)
	if err != nil {
		log.Error("Error whilst formatting %q in %s: %v", trKey, l.langName, err)
	}
	return msg
}

// Has returns whether a key is present in this locale or not
func (l *locale) Has(trKey string) bool {
	idx, ok := l.store.trKeyToIdxMap[trKey]
	if !ok {
		return false
	}
	_, ok = l.idxToMsgMap[idx]
	return ok
}
