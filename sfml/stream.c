#include <SFML/Audio/SoundStream.h>

// cgo export declarations
sfBool onStreamChunk(sfSoundStreamChunk* chunk, void* ptr);
void onStreamSeek(sfTime t, void* ptr);

// C callbacks
sfBool cgo_onStreamChunk(sfSoundStreamChunk* chunk, void* ptr) {
    return onStreamChunk((void*)chunk, ptr);
}

void cgo_onStreamSeek(sfTime time, void* ptr) {
    onStreamSeek(time, ptr);
}

// create a sfSoundStream using the callbacks above.
sfSoundStream* cgo_createStream(unsigned int channelCount, 
                                unsigned int sampleRate, 
                                void* obj)
{
    return sfSoundStream_create(cgo_onStreamChunk, 
                                cgo_onStreamSeek, 
                                channelCount, 
                                sampleRate, 
                                obj);
}
