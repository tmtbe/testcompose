package compose

import (
	"github.com/smartystreets/goconvey/convey"
	"testing"
)

func Test_Depend(t *testing.T) {
	convey.Convey("test depend", t, func() {
		floors, err := BuildDependFloors([]*PodConfig{
			{
				Name:    "A",
				Depends: []string{"B", "C"},
			},
			{
				Name:    "B",
				Depends: []string{"C"},
			},
			{
				Name:    "C",
				Depends: []string{"D", "F", "G"},
			},
			{
				Name:    "D",
				Depends: []string{},
			},
			{
				Name:    "E",
				Depends: []string{},
			},
			{
				Name:    "F",
				Depends: []string{"G"},
			},
			{
				Name:    "G",
				Depends: []string{},
			},
		})
		convey.So(err, convey.ShouldBeNil)
		convey.So(floors, convey.ShouldNotBeNil)
		order := floors.GetStartOrder()
		convey.So(order[4][0].Name, convey.ShouldEqual, "A")
		convey.So(order[3][0].Name, convey.ShouldEqual, "B")
	})

}
