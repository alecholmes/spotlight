package templates

import (
	"fmt"
	"html/template"

	"github.com/alecholmes/spotlight/util"
)

func parse(templateName string) *template.Template {
	resourcePath := util.ResourcePath(fmt.Sprintf("app/templates/html/%s.gohtml", templateName))
	return template.Must(template.New(templateName).ParseFiles(resourcePath))
}

func extend(tmpl *template.Template, templateName string) *template.Template {
	resourcePath := util.ResourcePath(fmt.Sprintf("app/templates/html/%s.gohtml", templateName))
	return template.Must(template.Must(tmpl.Clone()).ParseFiles(resourcePath))
}
