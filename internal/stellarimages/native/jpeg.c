#include <stddef.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <jpeglib.h>
static void u32(uint32_t v){unsigned char b[4]={v,v>>8,v>>16,v>>24};fwrite(b,1,4,stdout);}
static void fatal(j_common_ptr c){(*c->err->output_message)(c);exit(70);}
int main(void){unsigned char*data=malloc(32*1024*1024+1);if(!data)return 2;size_t len=fread(data,1,32*1024*1024+1,stdin);if(len>32*1024*1024)return 2;struct jpeg_decompress_struct c;struct jpeg_error_mgr e;c.err=jpeg_std_error(&e);e.error_exit=fatal;jpeg_create_decompress(&c);jpeg_mem_src(&c,data,len);if(jpeg_read_header(&c,TRUE)!=JPEG_HEADER_OK)return 3;uint32_t w=c.image_width,h=c.image_height;if(!w||!h||(uint64_t)w*h>40000000)return 4;double ratio=(double)(w>h?w:h)/64;int shrink=ratio>=8?8:ratio>=4?4:ratio>=2?2:1;if(shrink>1&&(int)ratio==shrink)shrink/=2;c.scale_num=1;c.scale_denom=shrink;c.do_fancy_upsampling=TRUE;c.dct_method=JDCT_ISLOW;int cmyk=c.jpeg_color_space==JCS_CMYK||c.jpeg_color_space==JCS_YCCK;c.out_color_space=cmyk?JCS_CMYK:JCS_RGB;jpeg_start_decompress(&c);u32(w);u32(h);uint32_t dw=w/shrink,dh=h/shrink;if(!dw)dw=1;if(!dh)dh=1;u32(dw);u32(dh);u32(cmyk?4:3);unsigned char*row=malloc((size_t)c.output_width*c.output_components);if(!row)return 4;while(c.output_scanline<c.output_height){JSAMPROW p=row;jpeg_read_scanlines(&c,&p,1);if(cmyk)for(size_t i=0;i<(size_t)c.output_width*4;i++)row[i]=255-row[i];if(c.output_scanline<=dh)fwrite(row,c.output_components,dw,stdout);}jpeg_finish_decompress(&c);jpeg_destroy_decompress(&c);free(row);free(data);return 0;}
