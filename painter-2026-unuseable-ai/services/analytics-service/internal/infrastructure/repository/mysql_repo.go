package repository

import (
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"painter-2026/analytics-service/internal/domain"
)

type MetricRecord struct {
	ID            uint `gorm:"primaryKey"`
	BucketDate    string
	QPS           int
	P95MS         int
	CacheHitRatio float64
	Visits        int
	CreatedAt     time.Time
}

type MySQLRepo struct {
	db *gorm.DB
}

func NewMySQLRepo(dsn string) (*MySQLRepo, error) {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&MetricRecord{}); err != nil {
		return nil, err
	}
	r := &MySQLRepo{db: db}
	if err := r.seed(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *MySQLRepo) seed() error {
	var count int64
	if err := r.db.Model(&MetricRecord{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	now := time.Now()
	rows := []MetricRecord{
		{BucketDate: now.AddDate(0, 0, -2).Format("2006-01-02"), QPS: 8, P95MS: 140, CacheHitRatio: 0.88, Visits: 120},
		{BucketDate: now.AddDate(0, 0, -1).Format("2006-01-02"), QPS: 11, P95MS: 132, CacheHitRatio: 0.9, Visits: 180},
		{BucketDate: now.Format("2006-01-02"), QPS: 13, P95MS: 118, CacheHitRatio: 0.93, Visits: 230},
	}
	return r.db.Create(&rows).Error
}

func (r *MySQLRepo) GetOverview() (domain.Overview, error) {
	var latest MetricRecord
	if err := r.db.Order("id desc").First(&latest).Error; err != nil {
		return domain.Overview{}, err
	}
	return domain.Overview{
		QPS:              latest.QPS,
		P95MS:            latest.P95MS,
		CacheHitRatio:    latest.CacheHitRatio,
		GeneratedAt:      time.Now().Format(time.RFC3339),
		SlowSQLThreshold: "200ms",
	}, nil
}

func (r *MySQLRepo) ListHistory(limit int) ([]map[string]interface{}, error) {
	if limit <= 0 || limit > 90 {
		limit = 30
	}
	var rows []MetricRecord
	if err := r.db.Order("id desc").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]interface{}{
			"date":          row.BucketDate,
			"qps":           row.QPS,
			"p95ms":         row.P95MS,
			"cacheHitRatio": row.CacheHitRatio,
			"visits":        row.Visits,
		})
	}
	return items, nil
}
