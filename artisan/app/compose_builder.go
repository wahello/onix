/*
  Onix Config Manager - Artisan
  Copyright (c) 2018-2021 by www.gatblau.org
  Licensed under the Apache License, Version 2.0 at http://www.apache.org/licenses/LICENSE-2.0
  Contributors to this project, hereby assign copyright in this code to the project,
  to be licensed under the same terms as the rest of the code.
*/

package app

import (
	"fmt"
	"github.com/compose-spec/compose-go/types"
	"gopkg.in/yaml.v2"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type ComposeBuilder struct {
	manifest Manifest
}

// newComposeBuilder called internally by NewBuilder()
func newComposeBuilder(appMan Manifest) Builder {
	return &ComposeBuilder{manifest: appMan}
}

func (b *ComposeBuilder) Build() ([]DeploymentRsx, error) {
	rsx := make([]DeploymentRsx, 0)
	composeProject, err := b.buildProject()
	if err != nil {
		return nil, err
	}
	rsx = append(rsx, *composeProject, b.buildEnv())
	files, err := b.buildFiles()
	if err != nil {
		return nil, err
	}
	svcScripts, err := b.buildInit()
	if err != nil {
		return nil, err
	}
	rsx = append(rsx, files...)
	return append(rsx, svcScripts...), nil
}

func (b *ComposeBuilder) buildProject() (*DeploymentRsx, error) {
	p := new(types.Project)
	p.Name = fmt.Sprintf("Docker Compose Project for %s", strings.ToUpper(b.manifest.Name))
	for _, svc := range b.manifest.Services {
		publishedPort, err := strconv.Atoi(svc.Port)
		if err != nil {
			return nil, fmt.Errorf("invalid published port '%s'\n", svc.Port)
		}
		targetPort, err := strconv.Atoi(svc.Info.Port)
		if err != nil {
			return nil, fmt.Errorf("invalid target port '%s'\n", svc.Port)
		}
		p.Services = append(p.Services, types.ServiceConfig{
			Name:          svc.Name,
			ContainerName: svc.Name,
			DependsOn:     getDeps(svc.DependsOn),
			Environment:   getEnv(svc.Info.Var),
			Image:         svc.Image,
			Ports:         []types.ServicePortConfig{{Target: uint32(targetPort), Published: uint32(publishedPort)}},
			Restart:       "always",
			Volumes:       append(getSvcVols(svc.Info.Volume), getFileVols(svc.Info.File)...),
		})
	}
	p.Volumes = getVols(b.manifest.Services)
	p.Networks = types.Networks{
		"default": types.NetworkConfig{
			Name: fmt.Sprintf("%s_network", strings.Replace(strings.ToLower(b.manifest.Name), " ", "_", -1)),
		},
	}
	composeProject, err := yaml.Marshal(p)
	if err != nil {
		return nil, err
	}
	return &DeploymentRsx{
		Name:    "docker-compose.yml",
		Content: composeProject,
		Type:    ComposeProject,
	}, nil
}

func (b *ComposeBuilder) buildEnv() DeploymentRsx {
	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf("# %s application environment file for docker-compose.yml\n", strings.ToUpper(b.manifest.Name)))
	builder.WriteString(fmt.Sprintf("# auto-generated by Onix Artisan on %s\n\n", time.Now().UTC()))
	sort.Slice(b.manifest.Var.Items, func(i, j int) bool {
		return b.manifest.Var.Items[i].Service < b.manifest.Var.Items[j].Service
	})
	currentSvc := ""
	for _, v := range b.manifest.Var.Items {
		if v.Service != currentSvc {
			builder.WriteString("# -----------------------------------------------------------------\n")
			builder.WriteString(fmt.Sprintf("# %s service\n", strings.ToUpper(v.Service)))
			builder.WriteString("# -----------------------------------------------------------------\n\n")
			currentSvc = v.Service
		}
		builder.WriteString(fmt.Sprintf("# %s \n", v.Description))
		builder.WriteString(fmt.Sprintf("%s=%s\n\n", v.Name, v.Value))
	}
	return DeploymentRsx{
		Name:    ".env",
		Content: []byte(builder.String()),
		Type:    EnvironmentFile,
	}
}

func (b ComposeBuilder) buildFiles() ([]DeploymentRsx, error) {
	rsx := make([]DeploymentRsx, 0)
	for _, svc := range b.manifest.Services {
		for _, f := range svc.Info.File {
			if len(f.Content) > 0 {
				rsx = append(rsx, DeploymentRsx{
					Name:    f.Path,
					Content: []byte(f.Content),
					Type:    ConfigurationFile,
				})
			} else {
				return nil, fmt.Errorf("definition of file '%s' in '%s' service manifest has no content\n", f.Path, svc.Name)
			}
		}
	}
	return rsx, nil
}

func (b ComposeBuilder) buildInit() ([]DeploymentRsx, error) {
	rsx := make([]DeploymentRsx, 0)
	for _, svc := range b.manifest.Services {
		var content []byte
		// if there is database schema configuration for the service
		if svc.Info.Db != nil {
			content = append(content, getDbScript(*svc.Info.Db)...)
		}
		// if there is specific initialisation logic for the service
		if len(svc.Info.Init) > 0 {
			content = append(content, []byte(svc.Info.Init)...)
		}
		if len(content) > 0 {
			rsx = append(rsx, DeploymentRsx{
				Name:    fmt.Sprintf("'%s' service initialisation", svc.Name),
				Content: content,
				Type:    SvcInitScript,
			})
		}
	}
	return rsx, nil
}

func getDbScript(db Db) []byte {
	s := new(strings.Builder)
	s.WriteString(fmt.Sprintf("# configure '%s' database release information\n", db.Name))
	s.WriteString(fmt.Sprintf("dbman config use %s-config\n", db.Name))
	s.WriteString(fmt.Sprintf("dbman config set SchemaURI %s\n", db.SchemaURI))
	s.WriteString(fmt.Sprintf("dbman config set db.provider %s\n", db.Provider))
	s.WriteString(fmt.Sprintf("dbman config set db.host %s\n", db.Host))
	s.WriteString(fmt.Sprintf("dbman config set db.port %d\n", db.Port))
	s.WriteString(fmt.Sprintf("dbman config set db.username %s\n", db.User))
	s.WriteString(fmt.Sprintf("dbman config set db.password %s\n", db.Pwd))
	s.WriteString(fmt.Sprintf("dbman config set db.adminusername %s\n", db.AdminUser))
	s.WriteString(fmt.Sprintf("dbman config set db.adminpassword %s\n", db.AdminPwd))
	s.WriteString(fmt.Sprintf("dbman config set db.appversion %s\n\n", db.AppVersion))
	s.WriteString(fmt.Sprintf("# create '%s' database\n", db.Name))
	s.WriteString(fmt.Sprintf("dbman db create\n\n"))
	s.WriteString(fmt.Sprintf("# deploy '%s' database schema\n", db.Name))
	s.WriteString(fmt.Sprintf("dbman db deploy\n\n"))
	return []byte(s.String())
}

func getSvcVols(volume []Volume) []types.ServiceVolumeConfig {
	vo := make([]types.ServiceVolumeConfig, 0)
	// does any explicit volumes
	for _, v := range volume {
		vo = append(vo, types.ServiceVolumeConfig{
			Extensions: map[string]interface{}{
				v.Name: v.Path,
			},
		})
	}
	return vo
}

// gets a list of volumes required by the specified files
func getFileVols(files []File) []types.ServiceVolumeConfig {
	vo := make([]types.ServiceVolumeConfig, 0)
	// does any explicit volumes
	for _, f := range files {
		relD := relDir(f.Path)
		found := false
		for _, x := range vo {
			if x.Extensions[relD] != nil {
				found = true
			}
		}
		if !found {
			vo = append(vo, types.ServiceVolumeConfig{
				Extensions: map[string]interface{}{
					relD: absDir(f.Path),
				},
			})
		}
	}
	return vo
}

func relDir(path string) string {
	// if the path is absolute
	if path[0] == '/' {
		// returns a relative form
		return fmt.Sprintf("./%s", filepath.Dir(path[1:]))
	}
	// if the path is not absolute but does not start with ./ add it
	if path[0:1] != "./" {
		return fmt.Sprintf("./%s", filepath.Dir(path[1:]))
	}
	// otherwise, return as is
	return filepath.Dir(path)
}

func absDir(path string) string {
	if path[0] == '/' {
		return filepath.Dir(path)
	}
	return filepath.Dir(fmt.Sprintf("/%s", filepath.Dir(path)))
}

func getDeps(dependencies []string) types.DependsOnConfig {
	d := types.DependsOnConfig{}
	for _, dependency := range dependencies {
		d[dependency] = types.ServiceDependency{Condition: types.ServiceConditionStarted}
	}
	return d
}

func getEnv(vars []Var) types.MappingWithEquals {
	var values []string
	for _, v := range vars {
		values = append(values, fmt.Sprintf("%s=%s", v.Name, v.Value))
	}
	return types.NewMappingWithEquals(values)
}

func getVols(svc []SvcRef) types.Volumes {
	vo := types.Volumes{}
	for _, s := range svc {
		for _, v := range s.Info.Volume {
			vo[v.Name] = types.VolumeConfig{
				External: types.External{External: true},
			}
		}
	}
	return vo
}
