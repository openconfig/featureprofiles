package fptest

import (
	"testing"
	"time"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ygnmi/ygnmi"
)

func TestDatapointValidator(t *testing.T) {
	tests := []struct {
		name    string
		dp      *ygnmi.DataPoint
		wantErr bool
	}{
		{
			name: "valid timestamp",
			dp: &ygnmi.DataPoint{
				Timestamp:     time.Unix(1707215426, 123456789),
				RecvTimestamp: time.Unix(1707215426, 123456790),
			},
			wantErr: false,
		},
		{
			name: "receive timestamp before notification timestamp",
			dp: &ygnmi.DataPoint{
				Timestamp:     time.Unix(1707215426, 123456789),
				RecvTimestamp: time.Unix(1707215426, 123456788),
			},
			wantErr: true,
		},
		{
			name: "zero timestamp",
			dp: &ygnmi.DataPoint{
				Timestamp:     time.Time{},
				RecvTimestamp: time.Unix(1707215426, 123456790),
			},
			wantErr: false,
		},
		{
			name: "valid UTF-8 string",
			dp: &ygnmi.DataPoint{
				Timestamp:     time.Unix(1707215426, 123456789),
				RecvTimestamp: time.Unix(1707215426, 123456790),
				Value:         &gpb.TypedValue{Value: &gpb.TypedValue_StringVal{StringVal: "hello"}},
			},
			wantErr: false,
		},
		{
			name: "invalid UTF-8 string",
			dp: &ygnmi.DataPoint{
				Timestamp:     time.Unix(1707215426, 123456789),
				RecvTimestamp: time.Unix(1707215426, 123456790),
				Value:         &gpb.TypedValue{Value: &gpb.TypedValue_StringVal{StringVal: "\xff\xfe\xfd"}},
			},
			wantErr: true,
		},
		{
			name: "empty string",
			dp: &ygnmi.DataPoint{
				Timestamp:     time.Unix(1707215426, 123456789),
				RecvTimestamp: time.Unix(1707215426, 123456790),
				Value:         &gpb.TypedValue{Value: &gpb.TypedValue_StringVal{StringVal: ""}},
			},
			wantErr: false,
		},
		{
			name: "non-string value",
			dp: &ygnmi.DataPoint{
				Timestamp:     time.Unix(1707215426, 123456789),
				RecvTimestamp: time.Unix(1707215426, 123456790),
				Value:         &gpb.TypedValue{Value: &gpb.TypedValue_IntVal{IntVal: 123}},
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := datapointValidator(tc.dp)
			if (err != nil) != tc.wantErr {
				t.Errorf("datapointValidator(%v) error = %v, wantErr %v", tc.dp, err, tc.wantErr)
			}
		})
	}
}
