package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/run-bigpig/jcp/internal/models"

	"github.com/google/uuid"
)

// SessionService Session服务
type SessionService struct {
	sessionsDir string
	sessions    map[string]*models.StockSession
	mu          sync.RWMutex
}

// NewSessionService 创建Session服务
func NewSessionService(dataDir string) *SessionService {
	ss := &SessionService{
		sessionsDir: filepath.Join(dataDir, "sessions"),
		sessions:    make(map[string]*models.StockSession),
	}
	ss.ensureDir()
	return ss
}

// ensureDir 确保目录存在
func (ss *SessionService) ensureDir() {
	if err := os.MkdirAll(ss.sessionsDir, 0755); err != nil {
		fmt.Printf("创建sessions目录失败: %v\n", err)
	}
}

func normalizeSessionStockCode(stockCode string) string {
	return strings.ToLower(strings.TrimSpace(stockCode))
}

func candidateStockCodes(stockCode string) []string {
	trimmed := strings.TrimSpace(stockCode)
	normalized := normalizeSessionStockCode(trimmed)
	candidates := make([]string, 0, 3)
	if normalized != "" {
		candidates = append(candidates, normalized)
		upper := strings.ToUpper(normalized)
		if upper != normalized {
			candidates = append(candidates, upper)
		}
	}
	if trimmed != "" && trimmed != normalized {
		candidates = append(candidates, trimmed)
	}
	seen := make(map[string]struct{}, len(candidates))
	unique := make([]string, 0, len(candidates))
	for _, item := range candidates {
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		unique = append(unique, item)
	}
	return unique
}

// getSessionPath 获取Session文件路径
func (ss *SessionService) getSessionPath(stockCode string) string {
	return filepath.Join(ss.sessionsDir, normalizeSessionStockCode(stockCode)+".json")
}

// GetOrCreateSession 获取或创建Session
func (ss *SessionService) GetOrCreateSession(stockCode, stockName string) (*models.StockSession, error) {
	code := normalizeSessionStockCode(stockCode)
	if code == "" {
		return nil, fmt.Errorf("stock code is empty")
	}
	ss.mu.Lock()
	defer ss.mu.Unlock()

	// 先从内存缓存获取
	if session, ok := ss.sessions[code]; ok {
		if strings.TrimSpace(stockName) != "" && strings.TrimSpace(session.StockName) == "" {
			session.StockName = strings.TrimSpace(stockName)
			session.UpdatedAt = time.Now().UnixMilli()
			_ = ss.saveSession(session)
		}
		return session, nil
	}

	// 尝试从文件加载
	session, err := ss.loadSession(code)
	if err == nil {
		if strings.TrimSpace(stockName) != "" && strings.TrimSpace(session.StockName) == "" {
			session.StockName = strings.TrimSpace(stockName)
			session.UpdatedAt = time.Now().UnixMilli()
			_ = ss.saveSession(session)
		}
		ss.sessions[code] = session
		return session, nil
	}

	// 创建新Session
	now := time.Now().UnixMilli()
	session = &models.StockSession{
		ID:        uuid.New().String(),
		StockCode: code,
		StockName: stockName,
		Messages:  []models.ChatMessage{},
		CreatedAt: now,
		UpdatedAt: now,
	}

	ss.sessions[code] = session
	return session, ss.saveSession(session)
}

// loadSession 从文件加载Session
func (ss *SessionService) loadSession(stockCode string) (*models.StockSession, error) {
	var data []byte
	var err error
	candidates := candidateStockCodes(stockCode)
	for _, candidate := range candidates {
		path := ss.getSessionPath(candidate)
		data, err = os.ReadFile(path)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("session not found: %s", stockCode)
	}

	var session models.StockSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}
	session.StockCode = normalizeSessionStockCode(session.StockCode)
	if session.StockCode == "" {
		session.StockCode = normalizeSessionStockCode(stockCode)
	}
	if session.Messages == nil {
		session.Messages = []models.ChatMessage{}
	}
	return &session, nil
}

// saveSession 保存Session到文件
func (ss *SessionService) saveSession(session *models.StockSession) error {
	if session == nil {
		return fmt.Errorf("session is nil")
	}
	session.StockCode = normalizeSessionStockCode(session.StockCode)
	if session.StockCode == "" {
		return fmt.Errorf("stock code is empty")
	}
	path := ss.getSessionPath(session.StockCode)
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// GetSession 获取Session
func (ss *SessionService) GetSession(stockCode string) *models.StockSession {
	code := normalizeSessionStockCode(stockCode)
	if code == "" {
		return nil
	}
	ss.mu.Lock()
	defer ss.mu.Unlock()

	// 先从内存缓存获取
	if session, ok := ss.sessions[code]; ok {
		return session
	}

	// 内存没有则尝试从文件加载
	session, err := ss.loadSession(code)
	if err != nil {
		return nil
	}

	ss.sessions[code] = session
	return session
}

// AddMessage 添加消息到Session
func (ss *SessionService) AddMessage(stockCode string, msg models.ChatMessage) error {
	code := normalizeSessionStockCode(stockCode)
	if code == "" {
		return fmt.Errorf("stock code is empty")
	}
	ss.mu.Lock()
	defer ss.mu.Unlock()

	session, ok := ss.sessions[code]
	if !ok {
		// 尝试从文件加载
		var err error
		session, err = ss.loadSession(code)
		if err != nil {
			return fmt.Errorf("session not found: %s", code)
		}
		ss.sessions[code] = session
	}

	msg.ID = uuid.New().String()
	msg.Timestamp = time.Now().UnixMilli()
	session.Messages = append(session.Messages, msg)
	session.UpdatedAt = time.Now().UnixMilli()
	return ss.saveSession(session)
}

// AddMessages 批量添加消息到Session
func (ss *SessionService) AddMessages(stockCode string, msgs []models.ChatMessage) error {
	code := normalizeSessionStockCode(stockCode)
	if code == "" {
		return fmt.Errorf("stock code is empty")
	}
	ss.mu.Lock()
	defer ss.mu.Unlock()

	session, ok := ss.sessions[code]
	if !ok {
		// 尝试从文件加载
		var err error
		session, err = ss.loadSession(code)
		if err != nil {
			return fmt.Errorf("session not found: %s", code)
		}
		ss.sessions[code] = session
	}

	now := time.Now().UnixMilli()
	for i := range msgs {
		msgs[i].ID = uuid.New().String()
		msgs[i].Timestamp = now
	}
	session.Messages = append(session.Messages, msgs...)
	session.UpdatedAt = now
	return ss.saveSession(session)
}

// GetMessages 获取Session消息
func (ss *SessionService) GetMessages(stockCode string) []models.ChatMessage {
	code := normalizeSessionStockCode(stockCode)
	if code == "" {
		return []models.ChatMessage{}
	}
	ss.mu.Lock()
	defer ss.mu.Unlock()

	// 先从内存缓存获取
	if session, ok := ss.sessions[code]; ok {
		return session.Messages
	}

	// 内存没有则尝试从文件加载
	session, err := ss.loadSession(code)
	if err != nil {
		return []models.ChatMessage{}
	}

	// 加载成功后缓存到内存
	ss.sessions[code] = session
	return session.Messages
}

// ClearMessages 清空Session消息
func (ss *SessionService) ClearMessages(stockCode string) error {
	code := normalizeSessionStockCode(stockCode)
	if code == "" {
		return fmt.Errorf("stock code is empty")
	}
	ss.mu.Lock()
	defer ss.mu.Unlock()

	session, ok := ss.sessions[code]
	if !ok {
		// 尝试从文件加载
		var err error
		session, err = ss.loadSession(code)
		if err != nil {
			return fmt.Errorf("session not found: %s", code)
		}
		ss.sessions[code] = session
	}

	session.Messages = []models.ChatMessage{}
	session.UpdatedAt = time.Now().UnixMilli()
	return ss.saveSession(session)
}

// UpdatePosition 更新持仓信息
func (ss *SessionService) UpdatePosition(stockCode string, shares int64, costPrice float64) error {
	code := normalizeSessionStockCode(stockCode)
	if code == "" {
		return fmt.Errorf("stock code is empty")
	}
	ss.mu.Lock()
	defer ss.mu.Unlock()

	session, ok := ss.sessions[code]
	if !ok {
		// 尝试从文件加载
		var err error
		session, err = ss.loadSession(code)
		if err != nil {
			now := time.Now().UnixMilli()
			session = &models.StockSession{
				ID:        uuid.New().String(),
				StockCode: code,
				StockName: code,
				Messages:  []models.ChatMessage{},
				CreatedAt: now,
				UpdatedAt: now,
			}
		}
		ss.sessions[code] = session
	}

	session.Position = &models.StockPosition{
		Shares:    shares,
		CostPrice: costPrice,
	}
	session.UpdatedAt = time.Now().UnixMilli()
	return ss.saveSession(session)
}

// GetPosition 获取持仓信息
func (ss *SessionService) GetPosition(stockCode string) *models.StockPosition {
	code := normalizeSessionStockCode(stockCode)
	if code == "" {
		return nil
	}
	ss.mu.Lock()
	defer ss.mu.Unlock()

	session, ok := ss.sessions[code]
	if !ok {
		// 尝试从文件加载
		session, err := ss.loadSession(code)
		if err != nil {
			return nil
		}
		ss.sessions[code] = session
		return session.Position
	}
	return session.Position
}
