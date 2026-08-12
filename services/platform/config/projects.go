package config

// ProjectsConfig holds project root directories and explicit project paths.
type ProjectsConfig struct {
	ProjectRoots []string `toml:"project_roots"`
	ProjectPaths []string `toml:"project_paths"`
}

// ListProjects returns all project directories.
func (p *ProjectsConfig) ListProjects() []string {
	return listProjectsFrom(p.ProjectRoots, p.ProjectPaths)
}
