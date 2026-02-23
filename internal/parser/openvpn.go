package parser

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Client представляет подключённого VPN-клиента
type Client struct {
	CommonName    string    `json:"common_name"`
	RealAddress   string    `json:"real_address"`
	VirtualAddr   string    `json:"virtual_address,omitempty"`
	BytesReceived int64     `json:"bytes_received"`
	BytesSent     int64     `json:"bytes_sent"`
	ConnectedSince time.Time `json:"connected_since"`
}

// Status содержит распарсенные данные из status-файла OpenVPN
type Status struct {
	UpdatedAt   time.Time `json:"updated_at"`
	Clients     []Client  `json:"clients"`
	GlobalStats *GlobalStats `json:"global_stats,omitempty"`
}

// GlobalStats глобальная статистика OpenVPN
type GlobalStats struct {
	MaxBcastMcastQueueLen int `json:"max_bcast_mcast_queue_len"`
}

var timeFormat = "Mon Jan 2 15:04:05 2006"
var timeFormatSpace = "Mon Jan  2 15:04:05 2006"

// ParseFile читает и парсит OpenVPN status-файл
func ParseFile(path string) (*Status, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("чтение файла: %w", err)
	}
	return ParseBytes(data)
}

// ParseBytes парсит содержимое status-файла без аллокации строки
func ParseBytes(data []byte) (*Status, error) {
	s := &Status{Clients: make([]Client, 0, 16)}
	lines := bytes.Split(data, []byte{'\n'})
	var inClientList bool
	var clientColumns []string

	for _, raw := range lines {
		line := bytes.TrimSpace(raw)
		if len(line) == 0 {
			continue
		}
		lineStr := string(line)

		if bytes.HasPrefix(bytes.ToUpper(line), []byte("OPENVPN CLIENT LIST")) {
			inClientList = true
			clientColumns = nil
			continue
		}

		if inClientList {
			if bytes.HasPrefix(line, []byte("Updated,")) {
				if _, t, ok := bytes.Cut(line, []byte(",")); ok {
					if parsed, err := parseOpenVPNTime(string(bytes.TrimSpace(t))); err == nil {
						s.UpdatedAt = parsed
					}
				}
				continue
			}

			if bytes.Contains(line, []byte("Common Name")) {
				clientColumns = parseColumns(lineStr)
				continue
			}

			if bytes.HasPrefix(bytes.ToUpper(line), []byte("ROUTING TABLE")) ||
				bytes.HasPrefix(bytes.ToUpper(line), []byte("GLOBAL STATS")) ||
				bytes.Equal(line, []byte("END")) {
				inClientList = false
				continue
			}

			if clientColumns != nil && !bytes.HasPrefix(line, []byte("GLOBAL STATS")) {
				c := parseClientLine(lineStr, clientColumns)
				if c.CommonName != "" {
					s.Clients = append(s.Clients, c)
				}
			}
		}

		if bytes.HasPrefix(bytes.ToUpper(line), []byte("GLOBAL STATS")) {
			s.GlobalStats = &GlobalStats{}
		} else if s.GlobalStats != nil && bytes.Contains(line, []byte("Max bcast")) {
			_, val, _ := bytes.Cut(line, []byte(","))
			s.GlobalStats.MaxBcastMcastQueueLen, _ = strconv.Atoi(string(bytes.TrimSpace(val)))
		}
	}

	if s.UpdatedAt.IsZero() {
		s.UpdatedAt = time.Now().UTC()
	}
	return s, nil
}

// Parse — алиас для совместимости
func Parse(content string) (*Status, error) {
	return ParseBytes([]byte(content))
}

func parseColumns(line string) []string {
	cols := strings.Split(line, ",")
	result := make([]string, 0, len(cols))
	for _, c := range cols {
		result = append(result, strings.TrimSpace(strings.ToLower(c)))
	}
	return result
}

func parseClientLine(line string, columns []string) Client {
	values := strings.Split(line, ",")
	c := Client{}
	for i, col := range columns {
		if i >= len(values) {
			break
		}
		val := strings.TrimSpace(values[i])
		switch col {
		case "common name":
			c.CommonName = val
		case "real address":
			c.RealAddress = val
		case "virtual address":
			c.VirtualAddr = val
		case "bytes received":
			c.BytesReceived, _ = strconv.ParseInt(val, 10, 64)
		case "bytes sent":
			c.BytesSent, _ = strconv.ParseInt(val, 10, 64)
		case "connected since":
			c.ConnectedSince, _ = parseOpenVPNTime(val)
		}
	}
	return c
}

func parseOpenVPNTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if t, err := time.Parse(timeFormat, s); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse(timeFormatSpace, s); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("неизвестный формат: %s", s)
}
