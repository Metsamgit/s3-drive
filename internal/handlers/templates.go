package handlers

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"io/fs"
)

// templates loads the HTML templates from an embedded filesystem.
// Embedding means the binary is self-contained — no template files to
// ship next to it, no template paths to escape to.
type templates struct {
	login    *template.Template
	files    *template.Template
	fileList *template.Template // partial used by HTMX swaps
}

func loadTemplates(fsys embed.FS) (*templates, error) {
	base, err := fs.Sub(fsys, "web/templates")
	if err != nil {
		return nil, err
	}

	parse := func(files ...string) (*template.Template, error) {
		t := template.New("base")
		for _, f := range files {
			b, err := fs.ReadFile(base, f)
			if err != nil {
				return nil, fmt.Errorf("read template %s: %w", f, err)
			}
			if _, err := t.Parse(string(b)); err != nil {
				return nil, fmt.Errorf("parse template %s: %w", f, err)
			}
		}
		return t, nil
	}

	tmpls := &templates{}
	if tmpls.login, err = parse("base.html", "login.html"); err != nil {
		return nil, err
	}
	if tmpls.files, err = parse("base.html", "files.html", "partials/file_list.html"); err != nil {
		return nil, err
	}
	if tmpls.fileList, err = parse("partials/file_list.html"); err != nil {
		return nil, err
	}
	return tmpls, nil
}

func (t *templates) renderLogin(w io.Writer, data any) error {
	return t.login.ExecuteTemplate(w, "base", data)
}

func (t *templates) renderFiles(w io.Writer, data any) error {
	return t.files.ExecuteTemplate(w, "base", data)
}

func (t *templates) renderFileList(w io.Writer, data any) error {
	return t.fileList.ExecuteTemplate(w, "file_list", data)
}
