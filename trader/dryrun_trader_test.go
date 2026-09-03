package trader

import "testing"

func TestDryRunTraderDoesNotCallInnerOnWrites(t *testing.T) {
	d := NewDryRunTrader(nil)
	if _, err := d.OpenLong("BTCUSDT", 1, 5); err != nil {
		t.Fatal(err)
	}
	if err := d.SetStopLoss("BTCUSDT", "LONG", 1, 1); err != nil {
		t.Fatal(err)
	}
	if len(d.Calls) != 2 {
		t.Fatalf("calls=%v", d.Calls)
	}
}
