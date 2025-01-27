package monitor

import (
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

func newPod(namespace, name string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
}

func addIP(in *corev1.Pod, ip string) *corev1.Pod {
	out := in.DeepCopy()
	out.Status.PodIPs = append(out.Status.PodIPs, corev1.PodIP{IP: ip})
	return out
}

func setDeletionTimestamp(in *corev1.Pod, deletionTime time.Time) *corev1.Pod {
	out := in.DeepCopy()
	out.DeletionTimestamp = &metav1.Time{Time: deletionTime}
	return out
}

func Test_podNetworkIPCache_updatePod(t *testing.T) {

	type fields struct {
		podIPsToCurrentPodLocators map[string]sets.String
	}
	type args struct {
		pod    *corev1.Pod
		oldPod *corev1.Pod
	}
	tests := []struct {
		name     string
		fields   fields
		args     args
		want     []monitorapi.Condition
		finalMap map[string]sets.String
	}{
		{
			name: "newly-created",
			fields: fields{
				podIPsToCurrentPodLocators: map[string]sets.String{},
			},
			args: args{
				pod: addIP(newPod("alfa", "zulu"), "10.28.0.96"),
			},
			want: nil,
			finalMap: map[string]sets.String{
				"10.28.0.96": sets.NewString(
					monitorapi.LocatePod(newPod("alfa", "zulu")),
				),
			},
		},
		{
			name: "deleted-not-present",
			fields: fields{
				podIPsToCurrentPodLocators: map[string]sets.String{},
			},
			args: args{
				pod: setDeletionTimestamp(addIP(newPod("alfa", "zulu"), "10.28.0.96"), time.Now()),
			},
			want:     nil,
			finalMap: map[string]sets.String{},
		},
		{
			name: "deleted-ip-present-not-with-pod",
			fields: fields{
				podIPsToCurrentPodLocators: map[string]sets.String{
					"10.28.0.96": sets.NewString(
						monitorapi.LocatePod(newPod("alfa", "yankee")),
					),
				},
			},
			args: args{
				pod: setDeletionTimestamp(addIP(newPod("alfa", "zulu"), "10.28.0.96"), time.Now()),
			},
			want: nil,
			finalMap: map[string]sets.String{
				"10.28.0.96": sets.NewString(
					monitorapi.LocatePod(newPod("alfa", "yankee")),
				),
			},
		},
		{
			name: "deleted-ip-present-with-pod",
			fields: fields{
				podIPsToCurrentPodLocators: map[string]sets.String{
					"10.28.0.96": sets.NewString(
						monitorapi.LocatePod(newPod("alfa", "zulu")),
					),
				},
			},
			args: args{
				pod: setDeletionTimestamp(addIP(newPod("alfa", "zulu"), "10.28.0.96"), time.Now()),
			},
			want:     nil,
			finalMap: map[string]sets.String{},
		},
		{
			name: "deleted-ip-present-with-two-pods",
			fields: fields{
				podIPsToCurrentPodLocators: map[string]sets.String{
					"10.28.0.96": sets.NewString(
						monitorapi.LocatePod(newPod("alfa", "yankee")),
						monitorapi.LocatePod(newPod("alfa", "zulu")),
					),
				},
			},
			args: args{
				pod: setDeletionTimestamp(addIP(newPod("alfa", "zulu"), "10.28.0.96"), time.Now()),
			},
			want: nil,
			finalMap: map[string]sets.String{
				"10.28.0.96": sets.NewString(
					monitorapi.LocatePod(newPod("alfa", "yankee")),
				),
			},
		},
		{
			name: "update-of-existing-pod",
			fields: fields{
				podIPsToCurrentPodLocators: map[string]sets.String{
					"10.28.0.96": sets.NewString(
						monitorapi.LocatePod(newPod("alfa", "zulu")),
					),
				},
			},
			args: args{
				pod: addIP(newPod("alfa", "zulu"), "10.28.0.96"),
			},
			want: nil,
			finalMap: map[string]sets.String{
				"10.28.0.96": sets.NewString(
					monitorapi.LocatePod(newPod("alfa", "zulu")),
				),
			},
		},
		{
			name: "update-of-existing-pod-already-duplicated",
			fields: fields{
				podIPsToCurrentPodLocators: map[string]sets.String{
					"10.28.0.96": sets.NewString(
						monitorapi.LocatePod(newPod("alfa", "yankee")),
						monitorapi.LocatePod(newPod("alfa", "zulu")),
					),
				},
			},
			args: args{
				pod: addIP(newPod("alfa", "zulu"), "10.28.0.96"),
			},
			want: nil,
			finalMap: map[string]sets.String{
				"10.28.0.96": sets.NewString(
					monitorapi.LocatePod(newPod("alfa", "yankee")),
					monitorapi.LocatePod(newPod("alfa", "zulu")),
				),
			},
		},
		{
			name: "add-conflicting-pod",
			fields: fields{
				podIPsToCurrentPodLocators: map[string]sets.String{
					"10.28.0.96": sets.NewString(
						monitorapi.LocatePod(newPod("alfa", "yankee")),
					),
				},
			},
			args: args{
				pod: addIP(newPod("alfa", "zulu"), "10.28.0.96"),
			},
			want: []monitorapi.Condition{
				{
					Level:   monitorapi.Error,
					Locator: monitorapi.LocatePod(newPod("alfa", "zulu")),
					Message: `reason/ReusedPodIP podIP 10.28.0.96 is currently assigned to multiple pods: ns/alfa pod/yankee node/ uid/;ns/alfa pod/zulu node/ uid/`,
				},
			},
			finalMap: map[string]sets.String{
				"10.28.0.96": sets.NewString(
					monitorapi.LocatePod(newPod("alfa", "yankee")),
					monitorapi.LocatePod(newPod("alfa", "zulu")),
				),
			},
		},
		{
			name: "add-conflicting-pod-second-ip",
			fields: fields{
				podIPsToCurrentPodLocators: map[string]sets.String{
					"10.28.0.96": sets.NewString(
						monitorapi.LocatePod(newPod("alfa", "yankee")),
					),
				},
			},
			args: args{
				pod: addIP(addIP(newPod("alfa", "zulu"), "2001:0db8:85a3:0000:0000:8a2e:0370:7334"), "10.28.0.96"),
			},
			want: []monitorapi.Condition{
				{
					Level:   monitorapi.Error,
					Locator: monitorapi.LocatePod(newPod("alfa", "zulu")),
					Message: `reason/ReusedPodIP podIP 10.28.0.96 is currently assigned to multiple pods: ns/alfa pod/yankee node/ uid/;ns/alfa pod/zulu node/ uid/`,
				},
			},
			finalMap: map[string]sets.String{
				"10.28.0.96": sets.NewString(
					monitorapi.LocatePod(newPod("alfa", "yankee")),
					monitorapi.LocatePod(newPod("alfa", "zulu")),
				),
				"2001:0db8:85a3:0000:0000:8a2e:0370:7334": sets.NewString(
					monitorapi.LocatePod(newPod("alfa", "zulu")),
				),
			},
		},
		{
			name: "change-pod-ip",
			fields: fields{
				podIPsToCurrentPodLocators: map[string]sets.String{
					"10.28.0.96": sets.NewString(
						monitorapi.LocatePod(newPod("alfa", "yankee")),
					),
				},
			},
			args: args{
				pod:    addIP(newPod("alfa", "yankee"), "2001:0db8:85a3:0000:0000:8a2e:0370:7334"),
				oldPod: addIP(newPod("alfa", "yankee"), "10.28.0.96"),
			},
			want: nil,
			finalMap: map[string]sets.String{
				"2001:0db8:85a3:0000:0000:8a2e:0370:7334": sets.NewString(
					monitorapi.LocatePod(newPod("alfa", "yankee")),
				),
			},
		},
		{
			name: "add-two-conflicts",
			fields: fields{
				podIPsToCurrentPodLocators: map[string]sets.String{
					"10.28.0.96": sets.NewString(
						monitorapi.LocatePod(newPod("alfa", "yankee")),
					),
					"2001:0db8:85a3:0000:0000:8a2e:0370:7334": sets.NewString(
						monitorapi.LocatePod(newPod("alfa", "x-ray")),
					),
				},
			},
			args: args{
				pod: addIP(addIP(newPod("alfa", "zulu"), "2001:0db8:85a3:0000:0000:8a2e:0370:7334"), "10.28.0.96"),
			},
			want: []monitorapi.Condition{
				{
					Level:   monitorapi.Error,
					Locator: monitorapi.LocatePod(newPod("alfa", "zulu")),
					Message: `reason/ReusedPodIP podIP 2001:0db8:85a3:0000:0000:8a2e:0370:7334 is currently assigned to multiple pods: ns/alfa pod/x-ray node/ uid/;ns/alfa pod/zulu node/ uid/`,
				},
				{
					Level:   monitorapi.Error,
					Locator: monitorapi.LocatePod(newPod("alfa", "zulu")),
					Message: `reason/ReusedPodIP podIP 10.28.0.96 is currently assigned to multiple pods: ns/alfa pod/yankee node/ uid/;ns/alfa pod/zulu node/ uid/`,
				},
			},
			finalMap: map[string]sets.String{
				"10.28.0.96": sets.NewString(
					monitorapi.LocatePod(newPod("alfa", "yankee")),
					monitorapi.LocatePod(newPod("alfa", "zulu")),
				),
				"2001:0db8:85a3:0000:0000:8a2e:0370:7334": sets.NewString(
					monitorapi.LocatePod(newPod("alfa", "x-ray")),
					monitorapi.LocatePod(newPod("alfa", "zulu")),
				),
			}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &podNetworkIPCache{
				podIPsToCurrentPodLocators: tt.fields.podIPsToCurrentPodLocators,
			}
			if got := p.updatePod(tt.args.pod, tt.args.oldPod); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("updatePod() = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(p.podIPsToCurrentPodLocators, tt.finalMap) {
				t.Errorf("map = %v, want %v", p.podIPsToCurrentPodLocators, tt.finalMap)
			}
		})
	}
}
