package compose

import "github.com/pkg/errors"

type DependFloor struct {
	FixWho map[string]*PodConfig
}
type DependFloors []*DependFloor

func (ds DependFloors) GetStartOrder() []map[string]*PodConfig {
	order := make([]map[string]*PodConfig, 0)
	filter := make(map[string]string, 0)
	for i := len(ds) - 1; i >= 0; i-- {
		orderFloor := make(map[string]*PodConfig)
		for k, v := range ds[i].FixWho {
			if _, ok := filter[k]; !ok {
				orderFloor[v.Name] = v
			}
			filter[k] = k
		}
		order = append(order, orderFloor)
	}
	return order
}
func NewDependFloor() *DependFloor {
	return &DependFloor{
		FixWho: make(map[string]*PodConfig, 0),
	}
}
func (df *DependFloor) buildNextFloor(dependFloors DependFloors, podMap map[string]*PodConfig) (DependFloors, error) {
	next := NewDependFloor()
	for _, pod := range df.FixWho {
		if len(pod.Depends) == 0 {
			next.FixWho[pod.Name] = pod
		}
		for _, podName := range pod.Depends {
			if find, ok := podMap[podName]; ok {
				next.FixWho[podName] = find
			} else {
				return nil, errors.New("An infinite loop of dependencies")
			}
		}
	}
	if len(next.FixWho) == len(df.FixWho) {
		return dependFloors, nil
	}
	dependFloors = append(dependFloors, next)
	return next.buildNextFloor(dependFloors, next.FixWho)
}

func BuildDependFloors(pods []*PodConfig) (DependFloors, error) {
	dependFloors := make([]*DependFloor, 0)
	dependFloorLevel0 := NewDependFloor()
	dependFloors = append(dependFloors, dependFloorLevel0)
	for _, pod := range pods {
		dependFloorLevel0.FixWho[pod.Name] = pod
	}
	dependFloors, err := dependFloorLevel0.buildNextFloor(dependFloors, dependFloorLevel0.FixWho)
	if err != nil {
		return nil, err
	}
	return dependFloors, nil
}
