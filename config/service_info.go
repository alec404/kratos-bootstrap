package config

import "os"

type ServiceInfo struct {
	Name     string
	Version  string
	Id       string
	Metadata map[string]string
}

func NewServiceInfo(name, version, id string) *ServiceInfo {
	if id == "" {
		id, _ = os.Hostname()
	}
	return &ServiceInfo{
		Name:     name,
		Version:  version,
		Id:       id,
		Metadata: map[string]string{},
	}
}

func (s *ServiceInfo) SetName(name string) {
	s.Name = name
}

func (s *ServiceInfo) SetVersion(version string) {
	s.Version = version
}

func (s *ServiceInfo) GetInstanceId() string {
	return s.Id + "." + s.Name
}

func (s *ServiceInfo) SetMataData(k, v string) {
	s.Metadata[k] = v
}

func (s *ServiceInfo) Clone() *ServiceInfo {
	if s == nil {
		return nil
	}
	metadata := make(map[string]string, len(s.Metadata))
	for k, v := range s.Metadata {
		metadata[k] = v
	}
	return &ServiceInfo{
		Name:     s.Name,
		Version:  s.Version,
		Id:       s.Id,
		Metadata: metadata,
	}
}
