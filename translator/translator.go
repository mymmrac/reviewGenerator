package translator

import (
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

const translateURL = "http://translate.googleapis.com/translate_a/single?client=gtx&sl=en&tl=uk&dt=t&q="

func TranslateText(text string) string {

	resp, err := http.Get(translateURL + url.QueryEscape(text))
	if err != nil {
		logrus.Fatal(err)
	}

	defer func() {
		err := resp.Body.Close()
		if err != nil {
			logrus.Fatal(err)
		}
	}()

	t, _ := ioutil.ReadAll(resp.Body)
	translated := string(t)

	r, _ := regexp.Compile("\".+?\"")

	match := r.FindAllString(translated, -1)

	var res string

	for i, m := range match {
		if i%4 == 0 {
			res += m
		}
	}

	return strings.Replace(strings.TrimSuffix(strings.ReplaceAll(res, "\"", ""), "en"), `\n`, "\n", -1)
}
