package lxc

type Remote struct {
	Name     string `yaml:"name,omitempty" json:"Name"`
	Addr     string `yaml:"addr,omitempty" json:"Addr"`
	AuthType string `yaml:"auth_type,omitempty" json:"AuthType"`
	Project  string `yaml:"project,omitempty" json:"Project"`
	Protocol string `yaml:"protocol,omitempty" json:"Protocol"`
	Public   bool   `yaml:"public,omitempty" json:"Public"`
	Global   bool   `yaml:"global,omitempty" json:"Global"`
	Static   bool   `yaml:"static,omitempty" json:"Static"`
}
