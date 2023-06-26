package main

// [START speech_transcribe_streaming_mic]
import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"

	speech "cloud.google.com/go/speech/apiv1"
	"cloud.google.com/go/speech/apiv1/speechpb"
	"github.com/gordonklaus/portaudio"
)

func float32ToByte(f []float32) []byte {
	var buf bytes.Buffer
	err := binary.Write(&buf, binary.BigEndian, f)
	if err != nil {
		fmt.Println("binary.Write failed:", err)
	}
	return buf.Bytes()
}

const sampleRate = 16000

func main() {
	ctx := context.Background()

	portaudio.Initialize()
	defer portaudio.Terminate()

	client, err := speech.NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}
	stream, err := client.StreamingRecognize(ctx)
	if err != nil {
		log.Fatal(err)
	}
	// Send the initial configuration message.
	if err := stream.Send(&speechpb.StreamingRecognizeRequest{
		StreamingRequest: &speechpb.StreamingRecognizeRequest_StreamingConfig{
			StreamingConfig: &speechpb.StreamingRecognitionConfig{
				Config: &speechpb.RecognitionConfig{
					Encoding:        speechpb.RecognitionConfig_LINEAR16,
					SampleRateHertz: sampleRate,
					LanguageCode:    "en-US",
				},
			},
		},
	}); err != nil {
		log.Fatal(err)
	}

	buf := make([]int32, 64)
	microphone, err := portaudio.OpenDefaultStream(1, 0, sampleRate, len(buf), buf)

	microphone.Start()
	defer microphone.Close()

	println("##")
	go func() {
		// Pipe stdin to the API.
		for {
			//n, err := os.Stdin.Read(buf)
			if err := microphone.Read(); err != nil {
				log.Fatalf("Read: %+v", err)
				break
			}
			var b = &bytes.Buffer{}
			err := binary.Write(b, binary.BigEndian, buf)
			if err != nil {
				panic(err)
			}
			if len(b.Bytes()) > 0 {
				if err := stream.Send(&speechpb.StreamingRecognizeRequest{
					StreamingRequest: &speechpb.StreamingRecognizeRequest_AudioContent{
						AudioContent: b.Bytes(),
					},
				}); err != nil {
					log.Printf("Could not send audio: %v", err)
				}
			}
		}
	}()

	for {
		println("read to recv")
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		println("recv")
		if err != nil {
			log.Fatalf("Cannot stream results: %v", err)
		}
		if err := resp.Error; err != nil {
			// Workaround while the API doesn't give a more informative error.
			if err.Code == 3 || err.Code == 11 {
				log.Print("WARNING: Speech recognition request exceeded limit of 60 seconds.")
			}
			log.Fatalf("Could not recognize: %v", err)
		}
		for _, result := range resp.Results {
			fmt.Printf("Result: %+v\n", result)
		}
	}
}
