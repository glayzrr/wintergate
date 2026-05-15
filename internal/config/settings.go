package config

// EndpointSettingsList 엔드포인트 설정 슬라이스 복제용 래퍼입니다.
type EndpointSettingsList []EndpointSettings

func (s ServiceSettings) Clone() ServiceSettings {
	return ServiceSettings{
		ServiceName: s.ServiceName,
		Global:      s.Global.Clone(),
		Threshold:   s.Threshold.Clone(),
		Endpoints:   EndpointSettingsList(s.Endpoints).Clone(),
		Instances:   append([]InstanceSettings(nil), s.Instances...),
	}
}

func (g *GlobalSettings) Clone() *GlobalSettings {
	if g == nil {
		return nil
	}
	return &GlobalSettings{
		Auth: g.Auth.Clone(),
	}
}

func (a *AuthSettings) Clone() *AuthSettings {
	if a == nil {
		return nil
	}
	return &AuthSettings{
		JWTAlgorithm: a.JWTAlgorithm,
		JWTAudience:  a.JWTAudience,
		JWTClockSkew: a.JWTClockSkew,
		JWTIssuer:    a.JWTIssuer,
		JWTSecret:    a.JWTSecret,
		JWKS:         append([]byte(nil), a.JWKS...),
	}
}

func (t *ThresholdSettings) Clone() *ThresholdSettings {
	if t == nil {
		return nil
	}
	return &ThresholdSettings{
		Normal: t.Normal,
		Hot:    t.Hot,
		Super:  t.Super,
	}
}

func (i *InstanceSettings) Clone() *InstanceSettings {
	if i == nil {
		return nil
	}
	return &InstanceSettings{
		Scheme: i.Scheme,
		Host:   i.Host,
		Port:   i.Port,
	}
}

func (e EndpointSettingsList) Clone() []EndpointSettings {
	if len(e) == 0 {
		return nil
	}

	clonedEndpoints := make([]EndpointSettings, 0, len(e))
	for _, endpoint := range e {
		clonedEndpoints = append(clonedEndpoints, EndpointSettings{
			Path:   endpoint.Path,
			Method: endpoint.Method,
			Roles:  append([]string(nil), endpoint.Roles...),
		})
	}

	return clonedEndpoints
}
