// Copyright (c) 2017 Cisco and/or its affiliates.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build !windows,!darwin

// Package vppapiclient is the default VPP adapter being used for the connection with VPP via shared memory.
// It is based on the communication with the vppapiclient VPP library written in C via CGO.
package vppapiclient

/*
#cgo CFLAGS: -DPNG_DEBUG=1
#cgo LDFLAGS: -lvppapiclient

#include <stdlib.h>
#include <stdio.h>
#include <stdint.h>
#include <arpa/inet.h>
#include <vpp-api/client/vppapiclient.h>

extern void go_msg_callback(uint16_t, uint32_t, void*, size_t);

typedef struct __attribute__((__packed__)) _req_header {
    uint16_t msg_id;
    uint32_t client_index;
    uint32_t context;
} req_header_t;

typedef struct __attribute__((__packed__)) _reply_header {
    uint16_t msg_id;
    uint32_t context;
} reply_header_t;

static void
govpp_msg_callback (unsigned char *data, int size)
{
    reply_header_t *header = ((reply_header_t *)data);
    go_msg_callback(ntohs(header->msg_id), ntohl(header->context), data, size);
}

static int
govpp_connect()
{
    return vac_connect("govpp", NULL, govpp_msg_callback, 32);
}

static int
govvp_disconnect()
{
    return vac_disconnect();
}

static int
govpp_send(uint32_t context, void *data, size_t size)
{
	req_header_t *header = ((req_header_t *)data);
	header->context = htonl(context);
    return vac_write(data, size);
}

static uint32_t
govpp_get_msg_index(char *name_and_crc)
{
    return vac_get_msg_index(name_and_crc);
}
*/
import "C"

import (
	"errors"
	"fmt"
	"reflect"
	"unsafe"

	"git.fd.io/govpp.git/adapter"
)

// vppAPIClientAdapter is the opaque context of the adapter.
type vppAPIClientAdapter struct {
	callback func(context uint32, msgId uint16, data []byte)
}

var vppClient *vppAPIClientAdapter // global vpp API client adapter context

// NewVppAdapter returns a new vpp API client adapter.
func NewVppAdapter() adapter.VppAdapter {
	return &vppAPIClientAdapter{}
}

// Connect connects the process to VPP.
func (a *vppAPIClientAdapter) Connect() error {
	vppClient = a
	rc := C.govpp_connect()
	if rc != 0 {
		return fmt.Errorf("unable to connect to VPP (error=%d)", rc)
	}
	return nil
}

// Disconnect disconnects the process from VPP.
func (a *vppAPIClientAdapter) Disconnect() {
	C.govvp_disconnect()
}

// GetMsgID returns a runtime message ID for the given message name and CRC.
func (a *vppAPIClientAdapter) GetMsgID(msgName string, msgCrc string) (uint16, error) {
	nameAndCrc := C.CString(fmt.Sprintf("%s_%s", msgName, msgCrc))
	defer C.free(unsafe.Pointer(nameAndCrc))

	msgID := uint16(C.govpp_get_msg_index(nameAndCrc))
	if msgID == ^uint16(0) {
		return msgID, errors.New("unkonwn message")
	}

	return msgID, nil
}

// SendMsg sends a binary-encoded message to VPP.
func (a *vppAPIClientAdapter) SendMsg(clientID uint32, data []byte) error {
	rc := C.govpp_send(C.uint32_t(clientID), unsafe.Pointer(&data[0]), C.size_t(len(data)))
	if rc != 0 {
		return fmt.Errorf("unable to send the message (error=%d)", rc)
	}
	return nil
}

// SetMsgCallback sets a callback function that will be called by the adapter whenever a message comes from VPP.
func (a *vppAPIClientAdapter) SetMsgCallback(cb func(context uint32, msgID uint16, data []byte)) {
	a.callback = cb
}

//export go_msg_callback
func go_msg_callback(msgID C.uint16_t, context C.uint32_t, data unsafe.Pointer, size C.size_t) {
	// convert unsafe.Pointer to byte slice
	slice := &reflect.SliceHeader{Data: uintptr(data), Len: int(size), Cap: int(size)}
	byteArr := *(*[]byte)(unsafe.Pointer(slice))

	vppClient.callback(uint32(context), uint16(msgID), byteArr)
}
