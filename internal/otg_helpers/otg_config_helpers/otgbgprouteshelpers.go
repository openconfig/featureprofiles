package otgconfighelpers

import (
	"fmt"

	"github.com/open-traffic-generator/snappi/gosnappi"
)

// RoutePrefix represents an IPv4 route.
type RoutePrefix struct {
	Address string
	Prefix  uint32
}

// PrefixFormat defines the format and length for generating prefixes.
type PrefixFormat struct {
	Format string
	Length uint32
}

// AdvertisementOptions contains options for a BGP route advertisement.
type AdvertisementOptions struct {
	TimeGap     uint32
	Origin      gosnappi.BgpAttributesOriginEnum
	ASPath      []uint32
	Communities map[uint32][]string
	NextHop     string
	LocalPref   *uint32
	IPv4Routes  []RoutePrefix
}

// WithdrawalOptions contains options for a BGP route withdrawal.
type WithdrawalOptions struct {
	TimeGap    uint32
	IPv4Routes []RoutePrefix
}

// AddBGPAdvertisement adds a structured BGP advertisement update to the list.
func AddBGPAdvertisement(updates gosnappi.BgpStructuredPdusBgpOneStructuredUpdateReplayIter, opts AdvertisementOptions) {
	adv := updates.Add()
	if opts.TimeGap > 0 {
		adv.SetTimeGap(opts.TimeGap)
	}

	pa := adv.PathAttributes()
	pa.SetOrigin(opts.Origin)

	if len(opts.ASPath) > 0 {
		pa.AsPath().FourByteAsPath().Segments().Add().
			SetType(gosnappi.BgpAttributesFourByteAsPathSegmentType.AS_SEQ).
			SetAsNumbers(opts.ASPath)
	}

	for asNum, customs := range opts.Communities {
		for _, custom := range customs {
			pa.Community().Add().CustomCommunity().SetAsNumber(asNum).SetCustom(custom)
		}
	}

	if opts.LocalPref != nil {
		pa.LocalPreference().SetValue(*opts.LocalPref)
	}

	mpReach := pa.MpReach()
	if opts.NextHop != "" {
		mpReach.NextHop().SetIpv4(opts.NextHop)
	}

	if len(opts.IPv4Routes) > 0 {
		ipv4Unicast := mpReach.Ipv4Unicast()
		for _, route := range opts.IPv4Routes {
			ipv4Unicast.Add().SetAddress(route.Address).SetPrefix(route.Prefix)
		}
	}
}

// AddBGPWithdrawal adds a structured BGP route withdrawal update to the list.
func AddBGPWithdrawal(updates gosnappi.BgpStructuredPdusBgpOneStructuredUpdateReplayIter, opts WithdrawalOptions) {
	wd := updates.Add()
	if opts.TimeGap > 0 {
		wd.SetTimeGap(opts.TimeGap)
	}

	if len(opts.IPv4Routes) > 0 {
		ipv4Unicast := wd.PathAttributes().MpUnreach().Ipv4Unicast()
		for _, route := range opts.IPv4Routes {
			ipv4Unicast.Add().SetAddress(route.Address).SetPrefix(route.Prefix)
		}
	}
}

// GenerateCommunityStrings creates a slice of community custom strings.
func GenerateCommunityStrings(count int, format string) []string {
	var communities []string
	for j := 0; j < count; j++ {
		communities = append(communities, fmt.Sprintf(format, j))
	}
	return communities
}

// GenerateIPv4Prefixes creates a slice of RoutePrefix based on formats.
func GenerateIPv4Prefixes(count int, formats []PrefixFormat) []RoutePrefix {
	var routes []RoutePrefix
	for j := 0; j <= count; j++ {
		for _, form := range formats {
			routes = append(routes, RoutePrefix{Address: fmt.Sprintf(form.Format, j), Prefix: form.Length})
		}
	}
	return routes
}
