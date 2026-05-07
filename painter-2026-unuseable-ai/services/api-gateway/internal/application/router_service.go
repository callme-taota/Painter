package application

import "os"

type RouterService struct{}

func NewRouterService() *RouterService {
	return &RouterService{}
}

func (s *RouterService) Resolve(domain string) string {
	switch domain {
	case "identity":
		return resolveEnv("IDENTITY_BASE_URL", "http://localhost:18081")
	case "content":
		return resolveEnv("CONTENT_BASE_URL", "http://localhost:18082")
	case "system":
		return resolveEnv("SYSTEM_BASE_URL", "http://localhost:18083")
	case "analytics":
		return resolveEnv("ANALYTICS_BASE_URL", "http://localhost:18084")
	default:
		return ""
	}
}

func resolveEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
