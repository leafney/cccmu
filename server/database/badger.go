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
		configs := map[string]interface{}{
			"config:cookie":    config.Cookie,
			"config:interval":  config.Interval,
			"config:timerange": config.TimeRange,
			"config:enabled":   config.Enabled,
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
		item, err := txn.Get([]byte("config:full"))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return nil // 返回默认配置
			}
			return err
		}

		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, config)
		})
	})

	return config, err
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
func (b *BadgerDB) GetUsageData(hours int) (models.UsageDataList, error) {
	var usageList models.UsageDataList

	err := b.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		it := txn.NewIterator(opts)
		defer it.Close()

		prefix := []byte("usage:")
		cutoff := time.Now().Add(-time.Duration(hours) * time.Hour).Unix()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			key := item.Key()

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
			}
		}

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