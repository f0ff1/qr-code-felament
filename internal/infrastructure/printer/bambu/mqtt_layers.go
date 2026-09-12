package bambu

import (
	"reflect"
	"unsafe"

	bambicloud "github.com/torbenconto/bambulabs_cloud_api"
	cloudmqtt "github.com/torbenconto/bambulabs_cloud_api/pkg/mqtt"
)

// layerFromCloudPool reads real layer_num from the cloud MQTT message.
// The upstream Data() mapping drops LayerNum and only keeps TotalLayerNumber,
// which previously forced us to invent current layer from print %.
func layerFromCloudPool(pool *bambicloud.PrinterPool, serial string) (current, total int, ok bool) {
	if pool == nil || serial == "" {
		return 0, 0, false
	}
	field := reflect.ValueOf(pool).Elem().FieldByName("mqttClient")
	if !field.IsValid() || field.IsNil() || !field.CanAddr() {
		return 0, 0, false
	}
	client, castOK := reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Interface().(*cloudmqtt.Client)
	if !castOK || client == nil {
		return 0, 0, false
	}
	msg := client.Data(serial)
	return msg.Print.LayerNum, msg.Print.TotalLayerNum, true
}
