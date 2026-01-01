package store

import (
	"database/sql"
	"encoding/json"

	"go.mau.fi/whatsmeow/types"
)

// Helper functions for null-safe SQL operations

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullJID(jid types.JID) sql.NullString {
	if jid.IsEmpty() {
		return sql.NullString{}
	}
	return sql.NullString{String: jid.String(), Valid: true}
}

func nullInt(i int) sql.NullInt64 {
	if i == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(i), Valid: true}
}

func nullInt64(i int64) sql.NullInt64 {
	if i == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: i, Valid: true}
}

func nullFloat(f float64) sql.NullFloat64 {
	if f == 0 {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: f, Valid: true}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func jidsToStrings(jids []types.JID) []string {
	result := make([]string, len(jids))
	for i, jid := range jids {
		result[i] = jid.String()
	}
	return result
}

func stringsToJIDs(strs []string) []types.JID {
	result := make([]types.JID, 0, len(strs))
	for _, s := range strs {
		if jid, err := types.ParseJID(s); err == nil {
			result = append(result, jid)
		}
	}
	return result
}

func jsonMarshal(v interface{}) []byte {
	if v == nil {
		return nil
	}
	b, _ := json.Marshal(v)
	return b
}

func jsonUnmarshalStrings(data []byte) []string {
	var result []string
	if len(data) > 0 {
		json.Unmarshal(data, &result)
	}
	return result
}

// Additional helpers for comprehensive coverage

func nullBool(b bool) sql.NullBool {
	return sql.NullBool{Bool: b, Valid: true}
}

func nullTime(t int64) sql.NullInt64 {
	if t == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: t, Valid: true}
}

func intToBool(i int) bool {
	return i != 0
}

func jidsToJSON(jids []types.JID) []byte {
	if len(jids) == 0 {
		return nil
	}
	strs := jidsToStrings(jids)
	b, _ := json.Marshal(strs)
	return b
}

func jidsFromJSON(data []byte) []types.JID {
	if len(data) == 0 {
		return nil
	}
	var strs []string
	if err := json.Unmarshal(data, &strs); err != nil {
		return nil
	}
	return stringsToJIDs(strs)
}

func parseJID(s string) types.JID {
	jid, _ := types.ParseJID(s)
	return jid
}

func parseNullJID(ns sql.NullString) types.JID {
	if !ns.Valid || ns.String == "" {
		return types.JID{}
	}
	return parseJID(ns.String)
}

func coalesceString(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func coalesceInt64(a, b int64) int64 {
	if a != 0 {
		return a
	}
	return b
}
