package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/runner-mei/gengen/types"
)

// GenerateViewCommand - 生成视图
type GenerateViewCommand struct {
	baseCommand
	layouts string
}

// Flags - 申明参数
func (cmd *GenerateViewCommand) Flags(fs *flag.FlagSet) *flag.FlagSet {
	fs.StringVar(&cmd.layouts, "layouts", "", "")
	return cmd.baseCommand.Flags(fs)
}

// Run - 生成代码
func (cmd *GenerateViewCommand) Run(args []string) error {
	return cmd.run(args, cmd.genrateView)
}

func (cmd *GenerateViewCommand) genrateView(cls *types.ClassSpec) error {
	ctlName := Pluralize(cls.Name)
	params := map[string]interface{}{"namespace": cmd.ns,
		"controllerName": ctlName,
		"modelName":      ctlName,
		"layouts":        cmd.layouts,
		"class":          cls}
	funcs := template.FuncMap{"localizeName": localizeName,
		"isClob": func(f types.FieldSpec) bool {
			if f.Restrictions != nil {
				if f.Restrictions.Length > 500 {
					return true
				}
				if f.Restrictions.MaxLength > 500 {
					return true
				}
			}
			return false
		},
		"isID": func(f types.FieldSpec) bool {
			if f.Name == "id" {
				return true
			}
			return false
		},
		"editDisabled": func(f types.FieldSpec) bool {
			for k, ann := range f.Annotations {
				if k == "editDisabled" {
					if v := strings.ToLower(fmt.Sprint(ann)); v == "true" || v == "yes" {
						return true
					}
				}
			}
			return false
		},
		"needDisplay": func(f types.FieldSpec) bool {
			for k, ann := range f.Annotations {
				if k == "noshow" {
					if v := strings.ToLower(fmt.Sprint(ann)); v == "true" || v == "yes" {
						return false
					}
				}
			}

			if f.Type == "password" {
				return false
			}
			return true
		}}

	err := cmd.executeTempate(cmd.override, []string{"views/index"}, funcs, params, filepath.Join(cmd.output, ctlName, "index.html"))
	if err != nil {
		return errors.New("gen views/index: " + err.Error())
	}

	err = cmd.executeTempate(cmd.override, []string{"views/edit"}, funcs, params, filepath.Join(cmd.output, ctlName, "edit.html"))
	if err != nil {
		os.Remove(filepath.Join(cmd.output, "index.html"))
		return errors.New("gen views/edit: " + err.Error())
	}

	err = cmd.executeTempate(cmd.override, []string{"views/fields"}, funcs, params, filepath.Join(cmd.output, ctlName, "edit_fields.html"))
	if err != nil {
		os.Remove(filepath.Join(cmd.output, "index.html"))
		os.Remove(filepath.Join(cmd.output, "edit.html"))
		return errors.New("gen views/fields: " + err.Error())
	}

	err = cmd.executeTempate(cmd.override, []string{"views/new"}, funcs, params, filepath.Join(cmd.output, ctlName, "new.html"))
	if err != nil {
		os.Remove(filepath.Join(cmd.output, "index.html"))
		os.Remove(filepath.Join(cmd.output, "edit.html"))
		os.Remove(filepath.Join(cmd.output, "edit_fields.html"))
		return errors.New("gen views/new: " + err.Error())
	}

	err = cmd.executeTempate(cmd.override, []string{"views/quick"}, funcs, params, filepath.Join(cmd.output, ctlName, "quick-bar.html"))
	if err != nil {
		os.Remove(filepath.Join(cmd.output, "index.html"))
		os.Remove(filepath.Join(cmd.output, "edit.html"))
		os.Remove(filepath.Join(cmd.output, "edit_fields.html"))
		os.Remove(filepath.Join(cmd.output, "new.html"))
		return errors.New("gen views/quick: " + err.Error())
	}
	return nil
}

func localizeName(f types.FieldSpec) string {
	if f.Label != "" {
		return f.Label
	}
	return f.Name
}
