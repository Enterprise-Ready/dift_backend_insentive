package tracing

import _ "github.com/PlatformCore/libpackage/observability/tracing"

type Provider struct{ Service string }

func NewProvider(service string) *Provider { return &Provider{Service: service} }
