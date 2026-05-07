package domain

type Overview struct {
	QPS              int     `json:"qps"`
	P95MS            int     `json:"p95ms"`
	CacheHitRatio    float64 `json:"cacheHitRatio"`
	GeneratedAt      string  `json:"generatedAt"`
	SlowSQLThreshold string  `json:"slowSQLThreshold"`
}
