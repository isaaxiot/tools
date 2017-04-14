package constants

import (
	"strings"
)

type Layout struct {
	Locales     []string
	Layout      string
	Description string
}

var layouts = []Layout{
	{[]string{"be"}, "by", "Belarussian"},
	{[]string{"bg"}, "bg", "Bulgarian"},
	{[]string{"bs"}, "croat", "Bosnian"},
	{[]string{"cs", "cs_CZ"}, "cz-lat2", "Czech"},
	{[]string{"de_CH", "de_LI"}, "sg-latin1", "Swiss German"},
	{[]string{"de", "de_DE", "en_DE"}, "de-latin1-nodeadkeys", "German (Latin1; no dead keys)"},
	{[]string{"da"}, "dk-latin1", "Danish"},
	{[]string{"en", "en_CA", "en_US", "en_AU", "zh", "eo", "ko", "us", "nl", "nl_NL", "ar", "fa", "hi", "id", "mg", "ml", "gu", "pa", "kn", "dz", "ne", "sq", "tl", "vi", "xh"}, "us", "American"},
	{[]string{"en_IE", "en_GB", "en_GG", "en_IM", "en_JE", "ga", "gd", "gv", "cy", "kw"}, "uk", "British"},
	{[]string{"xx"}, "dvorak", "Dvorak"},
	{[]string{"et"}, "et", "Estonian"},
	{[]string{"ast", "ca", "es", "eu", "gl"}, "es", "Spanish"},
	{[]string{"es_CL", "es_DO", "es_GT", "es_HN", "es_MX", "es_PA", "es_PE", "es_SV"}, "la-latin1", "Latin American"},
	{[]string{"fi"}, "fi-latin1", "Finnish"},
	{[]string{"fr", "fr_FR", "br", "oc"}, "fr-latin9", "French "},
	{[]string{"fr_BE", "nl_BE", "wa"}, "be2-latin1", "Belgian"},
	{[]string{"fr_CA"}, "cf", "Canadian French"},
	{[]string{"fr_CH"}, "fr_CH-latin1", "Swiss French"},
	{[]string{"el"}, "gr", "Greek"},
	{[]string{"he"}, "hebrew", "Hebrew"},
	{[]string{"hr"}, "croat", "Croatian"},
	{[]string{"hu"}, "hu", "Hungarian"},
	{[]string{"is", "en_IS"}, "is-latin1", "Icelandic"},
	{[]string{"it"}, "it", "Italian"},
	{[]string{"ky"}, "ky", "Kirghiz"},
	{[]string{"lt"}, "lt", "Lithuanian"},
	{[]string{"lv"}, "lv-latin4", "Latvian"},
	{[]string{"ja", "ja_JP"}, "jp106", "Japanese (106 Key)"},
	{[]string{"mk"}, "mk", "Macedonian"},
	{[]string{"no", "nn", "nb", "se"}, "no-latin1", "Norwegian"},
	{[]string{"pl"}, "pl", "Polish"},
	{[]string{"pt"}, "pt-latin1", "Portuguese (Latin-1)"},
	{[]string{"pt_BR"}, "br-latin1", "Brazilian (Standard)"},
	{[]string{"pt_BR"}, "br-abnt2", "Brazilian (Standard ABNT2)"},
	{[]string{"ro"}, "ro", "Romanian"},
	{[]string{"ru"}, "ru", "Russian"},
	{[]string{"sk"}, "sk-qwerty", "Slovakian"},
	{[]string{"sl"}, "slovene", "Slovenian"},
	{[]string{"sr", "sr@latin"}, "sr", "Serbian"},
	{[]string{"sv"}, "se-latin1", "Swedish"},
	{[]string{"th"}, "th-tis", "Thai"},
	{[]string{"ku", "tr"}, "trfu", "Turkish (F layout)"},
	{[]string{"ku", "tr"}, "trqu", "Turkish (Q layout)"},
	{[]string{"uk"}, "ua", "Ukrainian"},
}

func GetLayout(locale string) map[string]string {
	result := make(map[string]string)

	for _, layout := range layouts {
		for _, l := range layout.Locales {
			if strings.HasPrefix(l, locale) {
				result[layout.Description] = layout.Layout
				break
			}
		}
	}

	return result
}
