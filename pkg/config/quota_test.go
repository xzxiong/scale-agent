package config

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	apires "k8s.io/apimachinery/pkg/api/resource"
)

func TestCalculatedQuotaConfig(t *testing.T) {
	type args struct {
		quota Quota
	}
	tests := []struct {
		name string
		args args
		want QuotaConfig
	}{
		{
			name: "base",
			args: args{
				quota: Quota{
					Cpu:    apires.MustParse("1"),
					Memory: apires.MustParse("2Gi"),
				},
			},
			want: QuotaConfig{
				GOMaxProcs:    apires.MustParse("2"),
				MemoryCache:   apires.MustParse("2Gi"),
				DiskCache:     apires.MustParse("20Gi"),
				DiskIOPS:      1000,
				DiskBandWidth: apires.MustParse("100Mi"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculatedQuotaConfig(tt.args.quota)
			equal := func(name string, want, got apires.Quantity) {
				require.Equalf(t, want.String(), got.String(), "Quantity: %s", name)
			}
			require.Equal(t, got.GOMaxProcs.Value(), tt.want.GOMaxProcs.Value())
			equal("MemoryCache", got.MemoryCache, tt.want.MemoryCache)
			equal("DiskCache", got.DiskCache, tt.want.DiskCache)
			equal("DiskBandWidth", got.DiskBandWidth, tt.want.DiskBandWidth)
			require.Equal(t, tt.want.DiskIOPS, got.DiskIOPS)
		})
	}
}

// TestQuantity
// type_test.go:53: memorySize = 5368709120 (BinarySI), 5Gi
// type_test.go:57: memorySiz/10 = 536870912 (BinarySI), 512Mi
// type_test.go:60: diskSize = 5000000000 (DecimalSI), 5G
// type_test.go:63: milliCores = 5300 (DecimalSI), 5300m, value: 6
// type_test.go:66: milliCores = 5400 (DecimalSI), 5400m, value: 6
func TestQuantity(t *testing.T) {
	memorySize := apires.MustParse("5Gi")
	t.Logf("memorySize = %v (%v), %s\n", memorySize.Value(), memorySize.Format, memorySize.String())

	memorySize10 := memorySize.DeepCopy()
	memorySize10.Set(memorySize.Value() / 10)
	t.Logf("memorySiz/10 = %v (%v), %s\n", memorySize10.Value(), memorySize10.Format, memorySize10.String())

	diskSize := apires.MustParse("5G")
	t.Logf("diskSize = %v (%v), %s\n", diskSize.Value(), diskSize.Format, diskSize.String())

	cores := apires.MustParse("5300m")
	t.Logf("milliCores = %v (%v), %s, value: %v\n", cores.MilliValue(), cores.Format, cores.String(), cores.Value())

	cores2 := apires.MustParse("5.4")
	t.Logf("milliCores = %v (%v), %s, value: %v\n", cores2.MilliValue(), cores2.Format, cores2.String(), cores.Value())
}

func TestGenerated_QuotaConfig(t *testing.T) {
	type Spec struct {
		Quotas []GeneratedQuotaConfig `json:"quotas"`
	}

	q := Quota{
		Cpu:    apires.MustParse("1.5"),
		Memory: apires.MustParse("2Gi"),
	}
	data := &Spec{}
	data.Quotas = append(data.Quotas, NewGeneratedQuotaConfig(q, CalculatedQuotaConfig(q)))

	out, err := yaml.Marshal(data)
	t.Logf("config: %s, err: %v", out, err)

	configYaml := `quotas:
- cpu: 1500m
  memory: 2Gi
  goMaxProcs: 2000m
  memory_cache: 2Gi
  disk_cache: 20Gi
  disk_iops: 1000
  disk_bw: 100Mi
- cpu: 1500m
  memory: 2Gi
  goMaxProcs: 2
  memory_cache: 2Gi
  disk_cache: 20Gi
  disk_iops: 1000
  disk_bw: 100Mi
`
	inData := &Spec{}
	err = yaml.Unmarshal([]byte(configYaml), inData)
	require.NoError(t, err)

	equal := func(name string, want, got apires.Quantity) {
		require.Equalf(t, want.String(), got.String(), "Quantity: %s", name)
	}
	checkQuotaConfig := func(want QuotaConfig, got QuotaConfig) {
		require.Equal(t, want.GOMaxProcs.MilliValue(), got.GOMaxProcs.MilliValue())
		require.Equal(t, want.GOMaxProcs.Value(), got.GOMaxProcs.Value())
		equal("MemoryCache", want.MemoryCache, got.MemoryCache)
		equal("DiskCache", want.DiskCache, got.DiskCache)
		equal("DiskBandWidth", want.DiskBandWidth, got.DiskBandWidth)
		require.Equal(t, got.DiskIOPS, want.DiskIOPS)
	}
	for idx := range inData.Quotas {
		inData.Quotas[idx].Parse()
		t.Logf("parsed config[%d]: %v\n", idx, inData)
		checkQuotaConfig(data.Quotas[0].cfg, inData.Quotas[idx].cfg)
	}
}

func TestGenerated_Init(t *testing.T) {
	type Spec struct {
		Quotas []GeneratedQuotaConfig `json:"quotas"`
	}
	data := &Spec{}
	for idx := range allQuotas {
		data.Quotas = append(data.Quotas, NewGeneratedQuotaConfig(allQuotas[idx], CalculatedQuotaConfig(allQuotas[idx])))
	}

	out, err := yaml.Marshal(data)
	t.Logf("config: %s, err: %v", out, err)
}
