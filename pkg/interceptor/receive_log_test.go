package interceptor

import (
	"reflect"
	"testing"
)

func TestReceivedBuffer(t *testing.T) {
	for _, start := range []uint16{0, 1, 127, 128, 129, 511, 512, 513, 32767, 32768, 32769, 65407, 65408, 65409, 65534, 65535} {
		start := start

		rl, err := NewReceiveLog(128)
		if err != nil {
			t.Fatalf("%+v", err)
		}

		all := func(min uint16, max uint16) []uint16 {
			result := make([]uint16, 0)
			for i := min; i != max+1; i++ {
				result = append(result, i)
			}
			return result
		}
		join := func(parts ...[]uint16) []uint16 {
			result := make([]uint16, 0)
			for _, p := range parts {
				result = append(result, p...)
			}
			return result
		}

		add := func(nums ...uint16) {
			for _, n := range nums {
				seq := start + n
				rl.Add(seq)
			}
		}

		assertGet := func(nums ...uint16) {
			t.Helper()
			for _, n := range nums {
				seq := start + n
				if !rl.Get(seq) {
					t.Errorf("not found: %d", seq)
				}
			}
		}
		assertNOTGet := func(nums ...uint16) {
			t.Helper()
			for _, n := range nums {
				seq := start + n
				if rl.Get(seq) {
					t.Errorf("packet found for %d", seq)
				}
			}
		}
		assertMissing := func(skipLastN uint16, nums []uint16) {
			t.Helper()
			missing := rl.MissingSeqNumbers(skipLastN)
			if missing == nil {
				missing = []uint16{}
			}
			want := make([]uint16, 0, len(nums))
			for _, n := range nums {
				want = append(want, start+n)
			}
			if !reflect.DeepEqual(want, missing) {
				t.Errorf("missing want/got %v / %v", want, missing)
			}
		}
		assertLastConsecutive := func(lastConsecutive uint16) {
			want := lastConsecutive + start
			if rl.lastConsecutive != want {
				t.Errorf("invalid lastConsecutive want %d got %d", want, rl.lastConsecutive)
			}
		}

		add(0)
		assertGet(0)
		assertMissing(0, []uint16{})
		assertLastConsecutive(0) // first element added

		add(all(1, 127)...)
		assertGet(all(1, 127)...)
		assertMissing(0, []uint16{})
		assertLastConsecutive(127)

		add(128)
		assertGet(128)
		assertNOTGet(0)
		assertMissing(0, []uint16{})
		assertLastConsecutive(128)

		add(130)
		assertGet(130)
		assertNOTGet(1, 2, 129)
		assertMissing(0, []uint16{129})
		assertLastConsecutive(128)

		add(333)
		assertGet(333)
		assertNOTGet(all(0, 332)...)
		assertMissing(0, all(206, 332))  // all 127 elements missing before 333
		assertMissing(10, all(206, 323)) // skip last 10 packets (324-333) from check
		assertLastConsecutive(205)       // lastConsecutive is still out of the buffer

		add(329)
		assertGet(329)
		assertMissing(0, join(all(206, 328), all(330, 332)))
		assertMissing(5, join(all(206, 328))) // skip last 5 packets (329-333) from check
		assertLastConsecutive(205)

		add(all(207, 320)...)
		assertGet(all(207, 320)...)
		assertMissing(0, join([]uint16{206}, all(321, 328), all(330, 332)))
		assertLastConsecutive(205)

		add(334)
		assertGet(334)
		assertNOTGet(206)
		assertMissing(0, join(all(321, 328), all(330, 332)))
		assertLastConsecutive(320) // head of buffer is full of consecutive packages

		add(all(322, 328)...)
		assertGet(all(322, 328)...)
		assertMissing(0, join([]uint16{321}, all(330, 332)))
		assertLastConsecutive(320)

		add(321)
		assertGet(321)
		assertMissing(0, all(330, 332))
		assertLastConsecutive(329) // after adding a single missing packet, lastConsecutive should jump forward
	}
}
