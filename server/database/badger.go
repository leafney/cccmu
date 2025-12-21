package database

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/leafney/cccmu/server/models"
)

type BadgerDB struct {
	db *badger.DB
}

// NewBadgerDB 创建新的BadgerDB实例
func NewBadgerDB(path string) (*BadgerDB, error) {
	opts := badger.DefaultOptions(path).
		WithLoggingLevel(badger.WARNING)

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	return &BadgerDB{db: db}, nil
}

// Close 关闭数据库
func (b *BadgerDB) Close() error {
	return b.db.Close()
}

// SaveConfig 保存用户配置
func (b *BadgerDB) SaveConfig(config *models.UserConfig) error {
	return b.db.Update(func(txn *badger.Txn) error {
		data, err := json.Marshal(config)
		if err != nil {
			return err
		}

		// 保存各个配置项
		configs := map[string]any{
			"config:cookie":                   config.Cookie,
			"config:interval":                 config.Interval,
			"config:timerange":                config.TimeRange,
			"config:enabled":                  config.Enabled,
			"config:lastcookievalidtime":      config.LastCookieValidTime,
			"config:cookievalidationinterval": config.CookieValidationInterval,
			"config:autoschedule":             config.AutoSchedule,
		}

		for key, value := range configs {
			valueBytes, _ := json.Marshal(value)
			if err := txn.Set([]byte(key), valueBytes); err != nil {
				return err
			}
		}

		return txn.Set([]byte("config:full"), data)
	})
}

// GetConfig 获取用户配置
func (b *BadgerDB) GetConfig() (*models.UserConfig, error) {
	config := models.GetDefaultConfig()

	err := b.db.View(func(txn *badger.Txn) error {
		// 读取完整配置
		item, err := txn.Get([]byte("config:full"))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return nil // 返回默认配置
			}
			return err
		}

		err = item.Value(func(val []byte) error {
			return json.Unmarshal(val, config)
		})
		if err != nil {
			return err
		}

		// 单独读取cookie字段（因为Cookie字段有json:"-"标签，不会被序列化）
		cookieItem, err := txn.Get([]byte("config:cookie"))
		if err != nil && err != badger.ErrKeyNotFound {
			return err
		}
		if err == nil {
			err = cookieItem.Value(func(val []byte) error {
				var cookie string
				if unmarshalErr := json.Unmarshal(val, &cookie); unmarshalErr != nil {
					return unmarshalErr
				}
				config.Cookie = cookie
				return nil
			})
			if err != nil {
				return err
			}
		}

		return nil
	})

	return config, err
}

// ClearCookie 清除Cookie
func (b *BadgerDB) ClearCookie() error {
	return b.db.Update(func(txn *badger.Txn) error {
		// 删除单独存储的cookie键
		if err := txn.Delete([]byte("config:cookie")); err != nil && err != badger.ErrKeyNotFound {
			return err
		}

		// 获取完整配置并更新
		item, err := txn.Get([]byte("config:full"))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return nil // 配置不存在，无需清理
			}
			return err
		}

		var config *models.UserConfig
		err = item.Value(func(val []byte) error {
			config = &models.UserConfig{}
			return json.Unmarshal(val, config)
		})
		if err != nil {
			return err
		}

		// 清除cookie并保存更新后的配置
		config.Cookie = ""
		data, err := json.Marshal(config)
		if err != nil {
			return err
		}

		return txn.Set([]byte("config:full"), data)
	})
}

// SaveUsageData 保存积分使用数据
func (b *BadgerDB) SaveUsageData(data []models.UsageData) error {
	return b.db.Update(func(txn *badger.Txn) error {
		for _, usage := range data {
			key := fmt.Sprintf("usage:%d", usage.CreatedAt.Unix())
			value, err := json.Marshal(usage)
			if err != nil {
				return err
			}

			if err := txn.Set([]byte(key), value); err != nil {
				return err
			}
		}
		return nil
	})
}

// GetUsageData 获取指定时间范围内的积分使用数据
func (b *BadgerDB) GetUsageData(minutes int) (models.UsageDataList, error) {
	var usageList models.UsageDataList
	var totalCount int
	var filteredCount int

	err := b.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		it := txn.NewIterator(opts)
		defer it.Close()

		prefix := []byte("usage:")
		cutoff := time.Now().Add(-time.Duration(minutes) * time.Minute).Unix()
		log.Printf("数据查询: 时间范围=%d分钟, 截止时间=%s", minutes, time.Unix(cutoff, 0))

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			key := item.Key()
			totalCount++

			var usage models.UsageData
			err := item.Value(func(val []byte) error {
				return json.Unmarshal(val, &usage)
			})
			if err != nil {
				log.Printf("解析使用数据失败 %s: %v", key, err)
				continue
			}

			// 过滤时间范围
			if usage.CreatedAt.Unix() >= cutoff {
				usageList = append(usageList, usage)
				filteredCount++
				if filteredCount <= 3 {
					log.Printf("符合条件的数据: ID=%d, 积分=%d, 时间=%s", usage.ID, usage.CreditsUsed, usage.CreatedAt)
				}
			} else {
				if totalCount <= 3 {
					log.Printf("时间超出范围的数据: ID=%d, 积分=%d, 时间=%s", usage.ID, usage.CreditsUsed, usage.CreatedAt)
				}
			}
		}

		log.Printf("数据查询完成: 总数=%d, 符合条件=%d", totalCount, filteredCount)
		return nil
	})

	return usageList, err
}

// CleanOldData 清理过期数据
func (b *BadgerDB) CleanOldData(keepHours int) error {
	return b.db.Update(func(txn *badger.Txn) error {
		cutoff := time.Now().Add(-time.Duration(keepHours) * time.Hour).Unix()

		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()

		prefix := []byte("usage:")
		var keysToDelete [][]byte

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			key := item.Key()

			var usage models.UsageData
			err := item.Value(func(val []byte) error {
				return json.Unmarshal(val, &usage)
			})
			if err != nil {
				continue
			}

			if usage.CreatedAt.Unix() < cutoff {
				keysToDelete = append(keysToDelete, append([]byte(nil), key...))
			}
		}

		// 删除过期数据
		for _, key := range keysToDelete {
			if err := txn.Delete(key); err != nil {
				return err
			}
		}

		return nil
	})
}

// SaveCreditBalance 保存积分余额信息
func (b *BadgerDB) SaveCreditBalance(balance *models.CreditBalance) error {
	return b.db.Update(func(txn *badger.Txn) error {
		data, err := json.Marshal(balance)
		if err != nil {
			return err
		}

		return txn.Set([]byte("balance:latest"), data)
	})
}

// GetCreditBalance 获取积分余额信息
func (b *BadgerDB) GetCreditBalance() (*models.CreditBalance, error) {
	var balance *models.CreditBalance

	err := b.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("balance:latest"))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return nil // 返回nil表示未找到数据
			}
			return err
		}

		err = item.Value(func(val []byte) error {
			balance = &models.CreditBalance{}
			return json.Unmarshal(val, balance)
		})
		return err
	})

	return balance, err
}

// SaveDailyUsage 保存或累加每日积分使用统计
func (b *BadgerDB) SaveDailyUsage(date string, credits int) error {
	return b.db.Update(func(txn *badger.Txn) error {
		key := []byte(models.GetDailyUsageKey(date))

		// 尝试获取现有数据
		var currentUsage models.DailyUsage
		item, err := txn.Get(key)
		if err != nil && err != badger.ErrKeyNotFound {
			return err
		}

		if err == badger.ErrKeyNotFound {
			// 不存在，创建新记录
			currentUsage = models.DailyUsage{
				Date:         date,
				TotalCredits: credits,
			}
		} else {
			// 存在，累加积分
			err = item.Value(func(val []byte) error {
				return json.Unmarshal(val, &currentUsage)
			})
			if err != nil {
				return err
			}

			currentUsage.TotalCredits += credits
		}

		// 保存数据
		data, err := json.Marshal(currentUsage)
		if err != nil {
			return err
		}

		return txn.Set(key, data)
	})
}

// SaveDailyUsageWithModels 保存或累加每日积分使用统计（支持按模型分组）
func (b *BadgerDB) SaveDailyUsageWithModels(date string, credits int, modelCredits map[string]int) error {
	return b.db.Update(func(txn *badger.Txn) error {
		key := []byte(models.GetDailyUsageKey(date))

		// 尝试获取现有数据
		var currentUsage models.DailyUsage
		item, err := txn.Get(key)
		if err != nil && err != badger.ErrKeyNotFound {
			return err
		}

		if err == badger.ErrKeyNotFound {
			// 不存在，创建新记录
			currentUsage = models.DailyUsage{
				Date:         date,
				TotalCredits: credits,
				ModelCredits: make(map[string]int),
			}
			// 复制模型积分数据
			for model, modelCredit := range modelCredits {
				currentUsage.ModelCredits[model] = modelCredit
			}
		} else {
			// 存在，累加积分
			err = item.Value(func(val []byte) error {
				return json.Unmarshal(val, &currentUsage)
			})
			if err != nil {
				return err
			}

			// 确保 ModelCredits 字段不为 nil
			if currentUsage.ModelCredits == nil {
				currentUsage.ModelCredits = make(map[string]int)
			}

			// 累加总积分
			currentUsage.TotalCredits += credits

			// 按模型累加积分
			for model, modelCredit := range modelCredits {
				currentUsage.ModelCredits[model] += modelCredit
			}
		}

		// 保存数据
		data, err := json.Marshal(currentUsage)
		if err != nil {
			return err
		}

		return txn.Set(key, data)
	})
}

// GetDailyUsage 获取指定日期的积分使用统计
func (b *BadgerDB) GetDailyUsage(date string) (*models.DailyUsage, error) {
	var usage *models.DailyUsage

	err := b.db.View(func(txn *badger.Txn) error {
		key := []byte(models.GetDailyUsageKey(date))
		item, err := txn.Get(key)
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return nil // 返回nil表示未找到数据
			}
			return err
		}

		err = item.Value(func(val []byte) error {
			usage = &models.DailyUsage{}
			return json.Unmarshal(val, usage)
		})
		return err
	})

	// 确保 ModelCredits 字段不为 nil（兼容旧数据）
	if usage != nil && usage.ModelCredits == nil {
		usage.ModelCredits = make(map[string]int)
	}

	return usage, err
}

// GetWeeklyUsage 获取最近一周的每日积分使用统计
func (b *BadgerDB) GetWeeklyUsage() (models.DailyUsageList, error) {
	var usageList models.DailyUsageList

	err := b.db.View(func(txn *badger.Txn) error {
		weekDates := models.GetWeekDates()

		// 按日期获取数据
		for _, date := range weekDates {
			key := []byte(models.GetDailyUsageKey(date))

			item, err := txn.Get(key)
			if err != nil {
				if err == badger.ErrKeyNotFound {
					// 该日期没有数据，创建空记录
					usageList = append(usageList, models.DailyUsage{
						Date:         date,
						TotalCredits: 0,
						ModelCredits: make(map[string]int),
					})
					continue
				}
				return err
			}

			var usage models.DailyUsage
			err = item.Value(func(val []byte) error {
				return json.Unmarshal(val, &usage)
			})
			if err != nil {
				log.Printf("解析每日使用统计失败 %s: %v", key, err)
				continue
			}

			// 确保 ModelCredits 字段不为 nil（兼容旧数据）
			if usage.ModelCredits == nil {
				usage.ModelCredits = make(map[string]int)
			}

			usageList = append(usageList, usage)
		}

		log.Printf("获取一周积分统计完成: 共%d天数据", len(usageList))
		return nil
	})

	return usageList, err
}

// CleanupOldDailyUsage 清理超过指定天数的每日积分统计数据
func (b *BadgerDB) CleanupOldDailyUsage(keepDays int) error {
	return b.db.Update(func(txn *badger.Txn) error {
		cutoffDate := time.Now().Local().AddDate(0, 0, -keepDays).Format("2006-01-02")

		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()

		prefix := []byte("daily_usage:")
		var keysToDelete [][]byte
		var deletedCount int

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			key := item.Key()

			var usage models.DailyUsage
			err := item.Value(func(val []byte) error {
				return json.Unmarshal(val, &usage)
			})
			if err != nil {
				log.Printf("解析每日使用统计失败 %s: %v", key, err)
				continue
			}

			// 删除超过保留期限的数据
			if usage.Date < cutoffDate {
				keysToDelete = append(keysToDelete, append([]byte(nil), key...))
				deletedCount++
			}
		}

		// 执行删除操作
		for _, key := range keysToDelete {
			if err := txn.Delete(key); err != nil {
				return err
			}
		}

		if deletedCount > 0 {
			log.Printf("清理过期的每日积分统计: 删除%d条记录（保留%d天）", deletedCount, keepDays)
		}

		return nil
	})
}
