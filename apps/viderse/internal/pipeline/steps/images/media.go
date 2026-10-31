package images

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
	"os"
	"time"
)

// EncodeCover reuses the pipeline WebP encoder without drawing or inventing a receipt.
func (h Handler) EncodeCover(ctx context.Context,s *pipeline.StepContext,input,output string)(Encoded,error){return h.encode(ctx,s,input,output,1280,720)}
func (h Handler) encode(ctx context.Context, s *pipeline.StepContext, input, output string, width, height int) (Encoded, error) {
	if s.ProcessRunner == nil || h.Deps.Ffmpeg == "" || h.Deps.FFprobe == "" {
		return Encoded{}, steps.CloneError(domain.CloneErrToolMissing, "Thiếu FFmpeg/libwebp", "Kiểm tra FFmpeg trong Cài đặt")
	}
	_, err := s.RunProcess(ctx, h.Deps.Ffmpeg, []string{"-hide_banner", "-nostdin", "-y", "-i", input, "-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d", width, height, width, height), "-frames:v", "1", "-an", "-c:v", "libwebp", "-quality", "90", "-compression_level", "6", output}, tools.Spec{Timeout: 2 * time.Minute})
	if err != nil {
		return Encoded{}, err
	}
	data, err := os.ReadFile(output)
	if err != nil {
		return Encoded{}, err
	}
	if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		return Encoded{}, fmt.Errorf("output is not WebP")
	}
	r, err := s.RunProcess(ctx, h.Deps.FFprobe, []string{"-v", "error", "-show_streams", "-of", "json", output}, tools.Spec{Timeout: time.Minute})
	if err != nil {
		return Encoded{}, err
	}
	var probe struct {
		Streams []struct {
			Codec         string `json:"codec_name"`
			Width, Height int
		} `json:"streams"`
	}
	if err = json.Unmarshal([]byte(r.Stdout), &probe); err != nil {
		return Encoded{}, err
	}
	if len(probe.Streams) != 1 || probe.Streams[0].Codec != "webp" || probe.Streams[0].Width != width || probe.Streams[0].Height != height {
		return Encoded{}, fmt.Errorf("WebP dimensions/codec mismatch")
	}
	hash, err := steps.SHA256File(output)
	return Encoded{Width: width, Height: height, SHA256: hash, Bytes: int64(len(data))}, err
}

// macOS Vision's foreground-instance mask is the same local cutout backend
// as yt-factory. It never sends the paid image to another provider.
const cutoutScript = `ObjC.import('Foundation');ObjC.import('Vision');ObjC.import('AppKit');ObjC.import('CoreImage');
function run(argv) {
 var input=$.NSURL.fileURLWithPath(argv[0]);
 var handler=$.VNImageRequestHandler.alloc.initWithURLOptions(input,$({}));
 var request=$.VNGenerateForegroundInstanceMaskRequest.alloc.init;
 var error=Ref();
 if (!handler.performRequestsError($([request]),error)) throw Error('vision_failed');
 var observation=request.results.firstObject;
 if (!observation || observation.allInstances.count===0) throw Error('no_subject');
 var mask=observation.generateScaledMaskForImageForInstancesFromRequestHandlerError(observation.allInstances,handler,error);
 if (!mask) throw Error('vision_failed');
 var image=$.CIImage.imageWithContentsOfURL(input);
 var maskImage=$.CIImage.imageWithCVPixelBuffer(mask);
 var clear=$.CIImage.imageWithColor($.CIColor.colorWithRedGreenBlueAlpha(0,0,0,0));
 var filter=$.CIFilter.filterWithName('CIBlendWithMask');
 filter.setValueForKey(image,'inputImage');filter.setValueForKey(clear,'inputBackgroundImage');filter.setValueForKey(maskImage,'inputMaskImage');
 var output=filter.outputImage.imageByCroppingToRect(image.extent);
 var context=$.CIContext.contextWithOptions($({}));
 var cg=context.createCGImageFromRect(output,image.extent);
 var bitmap=$.NSBitmapImageRep.alloc.initWithCGImage(cg);
 var png=bitmap.representationUsingTypeProperties($.NSBitmapImageFileTypePNG,$({}));
 if (!png.writeToFileAtomically(argv[1],true)) throw Error('write_failed');
 return JSON.stringify({width:bitmap.pixelsWide,height:bitmap.pixelsHigh});
}`

func (h Handler) cutout(ctx context.Context, s *pipeline.StepContext, input, output string) error {
	if s.ProcessRunner == nil {
		return steps.CloneError(domain.CloneErrToolMissing, "Thiếu bộ tách nền", "Kiểm tra công cụ tách nền")
	}
	_, err := s.RunProcess(ctx, "/usr/bin/osascript", []string{"-l", "JavaScript", "-e", cutoutScript, input, output}, tools.Spec{Timeout: 2 * time.Minute})
	if err != nil {
		return steps.CloneError(domain.CloneErrInternal, "Không tách được nhân vật", "Không tìm thấy nhân vật để tách nền — bấm Vẽ lại")
	}
	return nil
}
