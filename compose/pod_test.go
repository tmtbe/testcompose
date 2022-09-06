package compose

import (
	"github.com/smartystreets/goconvey/convey"
	"testing"
)

func Test_FindDepend(t *testing.T) {
	pods := []*PodConfig{
		{
			Name:    "A",
			Depends: []string{"B"},
		},
		{
			Name:    "B",
			Depends: []string{"D"},
		},
		{
			Name:    "C",
			Depends: []string{"D"},
		},
		{
			Name:    "D",
			Depends: []string{},
		},
		{
			Name:    "E",
			Depends: []string{},
		},
	}
	convey.Convey("test depend", t, func() {
		compose, err := NewPodCompose("", pods, "", nil)
		convey.So(err, convey.ShouldBeNil)
		dependsPods := make(map[string]*PodConfig)
		dependsPods = compose.findWhoDependPods([]string{"D"}, dependsPods)
		convey.So(len(dependsPods), convey.ShouldEqual, 4)
		dependsPods = make(map[string]*PodConfig)
		dependsPods = compose.findWhoDependPods([]string{"E"}, dependsPods)
		convey.So(len(dependsPods), convey.ShouldEqual, 1)
	})
}
func Test_findPodsWhoUsedVolumes(t *testing.T) {
	pods := []*PodConfig{
		{
			Name: "A",
			Containers: []*ContainerConfig{
				{
					VolumeMounts: []*VolumeMountConfig{
						{
							Name: "work_dir",
						},
					},
				},
			},
		},
		{
			Name: "B",
			InitContainers: []*ContainerConfig{
				{
					VolumeMounts: []*VolumeMountConfig{
						{
							Name: "work_dir",
						},
					},
				},
			},
		},
	}
	convey.Convey("test find pods who used volumes", t, func() {
		compose, err := NewPodCompose("", pods, "", nil)
		convey.So(err, convey.ShouldBeNil)
		pods := compose.findPodsWhoUsedVolumes([]string{"work_dir"})
		convey.So(len(pods), convey.ShouldEqual, 2)
	})
}
