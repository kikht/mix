// Copyright (C) 2012-2014 by krepa098. All rights reserved.
// Use of this source code is governed by a zlib-style
// license that can be found in the license.txt file.

package gosfml2

/*
#include <SFML/Audio/SoundRecorder.h>

sfSoundRecorder* sfSoundRecorder_createEx(void*);
*/
import "C"

import (
	"runtime"
	"time"
	"unsafe"
)

/////////////////////////////////////
///		STRUCTS
/////////////////////////////////////

type SoundRecorder struct {
	cptr             *C.sfSoundRecorder
	startCallback    SoundRecorderCallbackStart
	stopCallback     SoundRecorderCallbackStop
	progressCallback SoundRecorderCallbackProgress
	userData         interface{}
}

type SoundRecorderCallbackStart func(userData interface{}) bool
type SoundRecorderCallbackStop func(userData interface{})
type SoundRecorderCallbackProgress func(samples []int16, userData interface{}) bool

/////////////////////////////////////
///		FUNCS
/////////////////////////////////////

// Construct a new sound recorder from callback functions
//
// 	onStart   Callback function which will be called when a new capture starts (can be nil)
// 	onProcess Callback function which will be called each time there's audio data to process (cannot be nil)
// 	onStop    Callback function which will be called when the current capture stops (can be nil)
// userData  Data to pass to the callback function (can be nil)
func NewSoundRecorder(onStart SoundRecorderCallbackStart, onProgress SoundRecorderCallbackProgress, onStop SoundRecorderCallbackStop, userData interface{}) (*SoundRecorder, error) {
	soundRecorder := &SoundRecorder{
		startCallback:    onStart,
		stopCallback:     onStop,
		progressCallback: onProgress,
		userData:         userData,
	}

	if cptr := C.sfSoundRecorder_createEx(unsafe.Pointer(soundRecorder)); cptr != nil {
		soundRecorder.cptr = cptr
		runtime.SetFinalizer(soundRecorder, (*SoundRecorder).destroy)
		return soundRecorder, nil
	}

	return nil, genericError
}

// Destroy an existing SoundRecorder
func (this *SoundRecorder) destroy() {
	C.sfSoundRecorder_destroy(this.cptr)
}

// The sampleRate parameter defines the number of audio samples
// captured per second. The higher, the better the quality
// (for example, 44100 samples/sec is CD quality).
// This function uses its own thread so that it doesn't block
// the rest of the program while the capture runs.
// Please note that only one capture can happen at the same time.
//
// 	sampleRate    Desired capture rate, in number of samples per second
func (this *SoundRecorder) Start(sampleRate uint) {
	C.sfSoundRecorder_start(this.cptr, C.uint(sampleRate))
}

// Stop the capture of a sound recorder
func (this *SoundRecorder) Stop() {
	C.sfSoundRecorder_stop(this.cptr)
}

// Get the sample rate of a sound recorder
//
// The sample rate defines the number of audio samples
// captured per second. The higher, the better the quality
// (for example, 44100 samples/sec is CD quality).
func (this *SoundRecorder) GetSampleRate() uint {
	return uint(C.sfSoundRecorder_getSampleRate(this.cptr))
}

// Set the processing interval
//
// The processing interval controls the period
// between calls to the onProcessSamples function. You may
// want to use a small interval if you want to process the
// recorded data in real time, for example.
//
// Note: this is only a hint, the actual period may vary.
// So don't rely on this parameter to implement precise timing.
//
// The default processing interval is 100 ms.
//
// 	interval Processing interval
func (this *SoundRecorder) SetProcessingInterval(interval time.Duration) {
	C.sfSoundRecorder_setProcessingInterval(this.cptr, C.sfTime{microseconds: C.sfInt64(interval.Nanoseconds() / 1000)})
}

// Check if the system supports audio capture
//
// This function should always be called before using
// the audio capture features. If it returns false, then
// any attempt to use SoundRecorder will fail.
func SoundRecorderIsAvailable() bool {
	return sfBool2Go(C.sfSoundRecorder_isAvailable())
}

/////////////////////////////////////
///		GO <-> C
/////////////////////////////////////

//export go_callbackStart
func go_callbackStart(ptr unsafe.Pointer) C.sfBool {
	soundRecoder := (*SoundRecorder)(ptr)
	if soundRecoder.startCallback != nil {
		return goBool2C(soundRecoder.startCallback(soundRecoder.userData))
	}
	return C.sfFalse //stop recording
}

//export go_callbackStop
func go_callbackStop(ptr unsafe.Pointer) {
	soundRecoder := (*SoundRecorder)(ptr)
	if soundRecoder.stopCallback != nil {
		soundRecoder.stopCallback(soundRecoder.userData)
	}
}

//export go_callbackProgress
func go_callbackProgress(data *C.sfInt16, count C.size_t, ptr unsafe.Pointer) C.sfBool {
	buffer := make([]int16, count)
	soundRecoder := (*SoundRecorder)(ptr)

	if len(buffer) > 0 {
		memcopy(unsafe.Pointer(&buffer[0]), unsafe.Pointer(data), len(buffer)*int(unsafe.Sizeof(int16(0))))
	}
	return goBool2C(soundRecoder.progressCallback(buffer, soundRecoder.userData))
}
