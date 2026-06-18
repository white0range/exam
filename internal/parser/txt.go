package parser

import "strings"

type BannerField struct {
	Key   string
	Value string
}

func TXTToBanner(records []string) map[string]string {
	fields := TXTFields(records)
	banner := make(map[string]string, len(fields))
	for _, field := range fields {
		banner[field.Key] = field.Value
	}
	return banner
}

func TXTFields(records []string) []BannerField {
	segments := splitTXTRecords(records)
	fields := make([]BannerField, 0, len(segments))
	for _, segment := range segments {
		key, value, ok := strings.Cut(segment, "=")
		if !ok {
			fields = append(fields, BannerField{Key: segment, Value: "true"})
			continue
		}
		fields = append(fields, BannerField{
			Key:   strings.TrimSpace(key),
			Value: strings.TrimSpace(value),
		})
	}
	return fields
}

func splitTXTRecords(records []string) []string {
	segments := make([]string, 0, len(records))
	for _, record := range records {
		record = strings.TrimSpace(record)
		if record == "" {
			continue
		}

		parts := strings.FieldsFunc(record, func(r rune) bool {
			return r == ',' || r == ';'
		})
		if len(parts) == 0 {
			continue
		}
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			segments = append(segments, part)
		}
	}
	return segments
}
