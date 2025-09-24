package datastore

import (
	"context"
	"fmt"
	"sort"
	"time"

	ds "github.com/ipfs/go-datastore"
)

// TTLStats - статистика по TTL ключам
type TTLStats struct {
	TotalKeys       int           `json:"total_keys"`
	ExpiredKeys     int           `json:"expired_keys"`
	ExpiringKeys    int           `json:"expiring_keys"` // истекают в ближайшие 5 минут
	KeysWithoutTTL  int           `json:"keys_without_ttl"`
	AverageTimeLeft time.Duration `json:"average_time_left"`
	NextExpiration  *time.Time    `json:"next_expiration,omitempty"`
}

// TTLKeyStatus - статус ключа с TTL
type TTLKeyStatus struct {
	Key       ds.Key        `json:"key"`
	ExpiresAt *time.Time    `json:"expires_at,omitempty"`
	TimeLeft  time.Duration `json:"time_left"`
	IsExpired bool          `json:"is_expired"`
	HasTTL    bool          `json:"has_ttl"`
}

// GetTTLStats - получает статистику по TTL ключам
func (s *datastorage) GetTTLStats(ctx context.Context, prefix ds.Key) (*TTLStats, error) {
	stats := &TTLStats{}

	keysCh, errCh, err := s.Keys(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения ключей: %w", err)
	}

	now := time.Now()
	var totalTimeLeft time.Duration
	keysWithTTL := 0
	var nextExp *time.Time

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case err, ok := <-errCh:
			if ok && err != nil {
				return nil, err
			}
		case key, ok := <-keysCh:
			if !ok {
				// Вычисляем средние значения
				if keysWithTTL > 0 {
					stats.AverageTimeLeft = totalTimeLeft / time.Duration(keysWithTTL)
				}
				stats.NextExpiration = nextExp
				return stats, nil
			}

			stats.TotalKeys++

			// Проверяем TTL для ключа
			expiration, err := s.Datastore.GetExpiration(ctx, key)
			if err != nil {
				// Ключ не имеет TTL
				stats.KeysWithoutTTL++
				continue
			}

			keysWithTTL++
			timeLeft := time.Until(expiration)
			totalTimeLeft += timeLeft

			if now.After(expiration) {
				stats.ExpiredKeys++
			} else {
				if timeLeft <= 5*time.Minute {
					stats.ExpiringKeys++
				}

				// Обновляем ближайшее истечение
				if nextExp == nil || expiration.Before(*nextExp) {
					nextExp = &expiration
				}
			}
		}
	}
}

// ListTTLKeys - возвращает список всех ключей с информацией о TTL
func (s *datastorage) ListTTLKeys(ctx context.Context, prefix ds.Key, onlyWithTTL bool) ([]TTLKeyStatus, error) {
	var results []TTLKeyStatus

	keysCh, errCh, err := s.Keys(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения ключей: %w", err)
	}

	now := time.Now()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case err, ok := <-errCh:
			if ok && err != nil {
				return nil, err
			}
		case key, ok := <-keysCh:
			if !ok {
				// Сортируем по времени истечения
				sort.Slice(results, func(i, j int) bool {
					if results[i].ExpiresAt == nil && results[j].ExpiresAt == nil {
						return results[i].Key.String() < results[j].Key.String()
					}
					if results[i].ExpiresAt == nil {
						return false
					}
					if results[j].ExpiresAt == nil {
						return true
					}
					return results[i].ExpiresAt.Before(*results[j].ExpiresAt)
				})
				return results, nil
			}

			status := TTLKeyStatus{
				Key: key,
			}

			// Проверяем TTL
			expiration, err := s.Datastore.GetExpiration(ctx, key)
			if err != nil {
				// Ключ не имеет TTL
				status.HasTTL = false
				status.TimeLeft = 0
				if !onlyWithTTL {
					results = append(results, status)
				}
				continue
			}

			status.HasTTL = true
			status.ExpiresAt = &expiration
			status.TimeLeft = time.Until(expiration)
			status.IsExpired = now.After(expiration)

			results = append(results, status)
		}
	}
}

// ExtendTTL - продлевает TTL для ключа
func (s *datastorage) ExtendTTL(ctx context.Context, key ds.Key, extension time.Duration) error {
	// Проверяем, что ключ существует и имеет TTL
	exists, err := s.Datastore.Has(ctx, key)
	if err != nil {
		return fmt.Errorf("ошибка проверки существования ключа: %w", err)
	}
	if !exists {
		return fmt.Errorf("ключ %s не существует", key.String())
	}

	currentExpiration, err := s.Datastore.GetExpiration(ctx, key)
	if err != nil {
		return fmt.Errorf("ошибка получения текущего TTL: %w", err)
	}

	// Вычисляем новый TTL
	now := time.Now()
	currentTTL := time.Until(currentExpiration)
	newTTL := currentTTL + extension

	// Если ключ уже истек, устанавливаем TTL от текущего момента
	if currentTTL <= 0 {
		newTTL = extension
	}

	err = s.Datastore.SetTTL(ctx, key, newTTL)
	if err != nil {
		return fmt.Errorf("ошибка установки нового TTL: %w", err)
	}

	// Обновляем в TTL мониторинге если он включен
	s.ttlMu.RLock()
	monitorEnabled := s.ttlMonitor != nil && s.ttlMonitor.Enabled
	s.ttlMu.RUnlock()

	if monitorEnabled {
		// Получаем текущее значение для обновления мониторинга
		value, err := s.Datastore.Get(ctx, key)
		if err == nil {
			s.registerTTLKey(key, now.Add(newTTL), value)
		}
	}

	return nil
}

// RefreshTTL - обновляет TTL ключа до исходного значения
func (s *datastorage) RefreshTTL(ctx context.Context, key ds.Key, originalTTL time.Duration) error {
	exists, err := s.Datastore.Has(ctx, key)
	if err != nil {
		return fmt.Errorf("ошибка проверки существования ключа: %w", err)
	}
	if !exists {
		return fmt.Errorf("ключ %s не существует", key.String())
	}

	err = s.Datastore.SetTTL(ctx, key, originalTTL)
	if err != nil {
		return fmt.Errorf("ошибка обновления TTL: %w", err)
	}

	// Обновляем в TTL мониторинге
	s.ttlMu.RLock()
	monitorEnabled := s.ttlMonitor != nil && s.ttlMonitor.Enabled
	s.ttlMu.RUnlock()

	if monitorEnabled {
		value, err := s.Datastore.Get(ctx, key)
		if err == nil {
			s.registerTTLKey(key, time.Now().Add(originalTTL), value)
		}
	}

	return nil
}

// CleanupExpiredKeys - принудительно очищает истекшие ключи
func (s *datastorage) CleanupExpiredKeys(ctx context.Context, prefix ds.Key) (int, error) {
	keysCh, errCh, err := s.Keys(ctx, prefix)
	if err != nil {
		return 0, fmt.Errorf("ошибка получения ключей: %w", err)
	}

	now := time.Now()
	expiredKeys := []ds.Key{}

	// Собираем истекшие ключи
	for {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case err, ok := <-errCh:
			if ok && err != nil {
				return 0, err
			}
		case key, ok := <-keysCh:
			if !ok {
				goto cleanup
			}

			expiration, err := s.Datastore.GetExpiration(ctx, key)
			if err != nil {
				// Ключ не имеет TTL, пропускаем
				continue
			}

			if now.After(expiration) {
				expiredKeys = append(expiredKeys, key)
			}
		}
	}

cleanup:
	// Удаляем истекшие ключи батчами
	if len(expiredKeys) == 0 {
		return 0, nil
	}

	batch, err := s.Batch(ctx)
	if err != nil {
		return 0, fmt.Errorf("ошибка создания batch: %w", err)
	}

	for _, key := range expiredKeys {
		// Получаем значение перед удалением для события
		var lastValue []byte
		if value, err := s.Datastore.Get(ctx, key); err == nil {
			lastValue = value
		}

		err = batch.Delete(ctx, key)
		if err != nil {
			return len(expiredKeys), fmt.Errorf("ошибка добавления в batch: %w", err)
		}

		// Генерируем событие TTL истечения
		s.publishTTLExpiredEvent(key, lastValue, now)
	}

	err = batch.Commit(ctx)
	if err != nil {
		return len(expiredKeys), fmt.Errorf("ошибка коммита batch: %w", err)
	}

	return len(expiredKeys), nil
}

// SetTTLBatch - устанавливает TTL для множества ключей одновременно
func (s *datastorage) SetTTLBatch(ctx context.Context, keys []ds.Key, ttl time.Duration) error {
	for _, key := range keys {
		err := s.Datastore.SetTTL(ctx, key, ttl)
		if err != nil {
			return fmt.Errorf("ошибка установки TTL для ключа %s: %w", key.String(), err)
		}

		// Обновляем в TTL мониторинге
		s.ttlMu.RLock()
		monitorEnabled := s.ttlMonitor != nil && s.ttlMonitor.Enabled
		s.ttlMu.RUnlock()

		if monitorEnabled {
			if value, err := s.Datastore.Get(ctx, key); err == nil {
				s.registerTTLKey(key, time.Now().Add(ttl), value)
			}
		}
	}

	return nil
}

// GetExpiringKeys - возвращает ключи, которые истекут в указанный период
func (s *datastorage) GetExpiringKeys(ctx context.Context, prefix ds.Key, within time.Duration) ([]TTLKeyStatus, error) {
	allKeys, err := s.ListTTLKeys(ctx, prefix, true) // только ключи с TTL
	if err != nil {
		return nil, err
	}

	var expiringKeys []TTLKeyStatus
	now := time.Now()

	for _, keyStatus := range allKeys {
		if keyStatus.HasTTL && keyStatus.ExpiresAt != nil {
			if keyStatus.ExpiresAt.After(now) && keyStatus.TimeLeft <= within {
				expiringKeys = append(expiringKeys, keyStatus)
			}
		}
	}

	return expiringKeys, nil
}

// ExportTTLReport - экспортирует отчет по TTL в JSON-совместимый формат
func (s *datastorage) ExportTTLReport(ctx context.Context, prefix ds.Key) (map[string]interface{}, error) {
	stats, err := s.GetTTLStats(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения статистики: %w", err)
	}

	keys, err := s.ListTTLKeys(ctx, prefix, false)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения списка ключей: %w", err)
	}

	// Группируем ключи по статусу
	keysByStatus := map[string][]string{
		"expired":       {},
		"expiring_soon": {},
		"with_ttl":      {},
		"without_ttl":   {},
	}

	now := time.Now()

	for _, keyStatus := range keys {
		keyStr := keyStatus.Key.String()

		if !keyStatus.HasTTL {
			keysByStatus["without_ttl"] = append(keysByStatus["without_ttl"], keyStr)
		} else if keyStatus.IsExpired {
			keysByStatus["expired"] = append(keysByStatus["expired"], keyStr)
		} else if keyStatus.TimeLeft <= 5*time.Minute {
			keysByStatus["expiring_soon"] = append(keysByStatus["expiring_soon"], keyStr)
		} else {
			keysByStatus["with_ttl"] = append(keysByStatus["with_ttl"], keyStr)
		}
	}

	config := s.GetTTLMonitorConfig()

	report := map[string]interface{}{
		"timestamp":      now.Format(time.RFC3339),
		"prefix":         prefix.String(),
		"stats":          stats,
		"keys_by_status": keysByStatus,
		"monitor_config": config,
	}

	return report, nil
}
