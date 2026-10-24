#include <webp/decode.h>
#include <webp/demux.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <math.h>
#include <string.h>
static void u32(uint32_t v){unsigned char b[4]={v,v>>8,v>>16,v>>24};fwrite(b,1,4,stdout);}
int main(void){unsigned char*data=malloc(32*1024*1024+1);if(!data)return 2;size_t len=fread(data,1,32*1024*1024+1,stdin);if(len>32*1024*1024)return 2;WebPDecoderConfig c;if(!WebPInitDecoderConfig(&c)||WebPGetFeatures(data,len,&c.input)!=VP8_STATUS_OK)return 3;int w=c.input.width,h=c.input.height;if(w<=0||h<=0||(uint64_t)w*h>40000000)return 4;double scale=(w>h?w:h)>64?64.0/(w>h?w:h):1.0;int dw=fmax(1,floor(w*scale+0.5)),dh=fmax(1,floor(h*scale+0.5));c.output.colorspace=MODE_RGBA;c.options.use_threads=0;c.options.use_scaling=scale<1;c.options.scaled_width=dw;c.options.scaled_height=dh;WebPData input={data,len};WebPDemuxer*demux=WebPDemux(&input);WebPIterator iterator;const unsigned char*image=data;size_t image_len=len;int have_frame=demux&&WebPDemuxGetFrame(demux,1,&iterator);if(have_frame){image=iterator.fragment.bytes;image_len=iterator.fragment.size;}
 int ox=0,oy=0;if(have_frame){ox=rint(iterator.x_offset*scale);oy=rint(iterator.y_offset*scale);c.options.scaled_width=fmax(1,rint(iterator.width*scale));c.options.scaled_height=fmax(1,rint(iterator.height*scale));}
 if(WebPDecode(image,image_len,&c)!=VP8_STATUS_OK)return 5;unsigned char*canvas=calloc((size_t)dw*dh,4);if(!canvas)return 4;for(int y=0;y<c.output.height&&y+oy<dh;y++){int n=c.output.width;if(n>dw-ox)n=dw-ox;if(n>0)memcpy(canvas+((size_t)(y+oy)*dw+ox)*4,c.output.u.RGBA.rgba+y*c.output.u.RGBA.stride,n*4);}u32(w);u32(h);u32(dw);u32(dh);fwrite(canvas,4,(size_t)dw*dh,stdout);free(canvas);WebPFreeDecBuffer(&c.output);if(have_frame)WebPDemuxReleaseIterator(&iterator);if(demux)WebPDemuxDelete(demux);free(data);return 0;}
