/* libtiff 4.7.1 内存读取器，不开放宿主文件、递归 IFD 或网络。 */
#include <tiffio.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
typedef struct {unsigned char *data;size_t len,pos;} memory;
static tmsize_t rd(thandle_t h,void *b,tmsize_t n){memory*m=h;if(n<0)return -1;if((size_t)n>m->len-m->pos)n=m->len-m->pos;memcpy(b,m->data+m->pos,n);m->pos+=n;return n;}
static tmsize_t wr(thandle_t h,void*b,tmsize_t n){return -1;}
static toff_t sk(thandle_t h,toff_t off,int whence){memory*m=h;toff_t p=whence==SEEK_SET?off:whence==SEEK_CUR?m->pos+off:m->len+off;if(p>m->len)return (toff_t)-1;m->pos=p;return p;}
static int cl(thandle_t h){return 0;}static toff_t sz(thandle_t h){return ((memory*)h)->len;}static int mp(thandle_t h,void **b,toff_t *s){memory*m=h;*b=m->data;*s=m->len;return 1;}static void um(thandle_t h,void*b,toff_t s){}
static void u32(uint32_t v){unsigned char b[4]={v,v>>8,v>>16,v>>24};fwrite(b,1,4,stdout);}
int main(void){
 memory m={malloc(32*1024*1024+1),0,0};if(!m.data)return 2;m.len=fread(m.data,1,32*1024*1024+1,stdin);if(m.len>32*1024*1024)return 2;
 TIFF*t=TIFFClientOpen("memory","rm",&m,rd,wr,sk,cl,sz,mp,um);if(!t)return 3;
 uint32_t w=0,h=0;uint16_t bits=8,spp=1,photo=0,planar=1,format=1,orientation=1;uint32_t plen=0;void*profile=NULL;
 TIFFGetField(t,TIFFTAG_IMAGEWIDTH,&w);TIFFGetField(t,TIFFTAG_IMAGELENGTH,&h);TIFFGetFieldDefaulted(t,TIFFTAG_BITSPERSAMPLE,&bits);TIFFGetFieldDefaulted(t,TIFFTAG_SAMPLESPERPIXEL,&spp);TIFFGetFieldDefaulted(t,TIFFTAG_PHOTOMETRIC,&photo);TIFFGetFieldDefaulted(t,TIFFTAG_PLANARCONFIG,&planar);TIFFGetFieldDefaulted(t,TIFFTAG_SAMPLEFORMAT,&format);TIFFGetFieldDefaulted(t,TIFFTAG_ORIENTATION,&orientation);TIFFGetField(t,TIFFTAG_ICCPROFILE,&plen,&profile);
 if(!w||!h||(uint64_t)w*h>40000000||spp>8||plen>32*1024*1024)return 4;
 // 调色板、单比特和打包像素使用 libtiff 自身 RGBA 读取路径。
 if(bits!=8&&bits!=16&&!(bits==32&&format==SAMPLEFORMAT_IEEEFP)||photo==PHOTOMETRIC_PALETTE||photo==PHOTOMETRIC_YCBCR){
  uint32_t*r=malloc((size_t)w*h*4);if(!r||!TIFFReadRGBAImageOriented(t,w,h,r,ORIENTATION_TOPLEFT,0))return 5;
  u32(w);u32(h);u32(PHOTOMETRIC_RGB);u32(8);u32(4);u32(1);u32(1);u32(plen);fwrite(profile,1,plen,stdout);
  for(size_t i=0;i<(size_t)w*h;i++){unsigned char b[4]={TIFFGetR(r[i]),TIFFGetG(r[i]),TIFFGetB(r[i]),TIFFGetA(r[i])};fwrite(b,1,4,stdout);}free(r);TIFFClose(t);free(m.data);return 0;
 }
 size_t sample=bits/8,rowbytes=(size_t)w*spp*sample,total=rowbytes*h;
 if(total>640000000)return 4;unsigned char*out=calloc(1,total);if(!out)return 4;
 if(TIFFIsTiled(t)){
  uint32_t tw=0,th=0;TIFFGetField(t,TIFFTAG_TILEWIDTH,&tw);TIFFGetField(t,TIFFTAG_TILELENGTH,&th);tmsize_t tilesize=TIFFTileSize(t);if(!tw||!th||tilesize<=0||tilesize>640000000)return 4;unsigned char*tile=malloc(tilesize);if(!tile)return 4;
  for(uint16_t s=0;s<(planar==PLANARCONFIG_SEPARATE?spp:1);s++)for(uint32_t y=0;y<h;y+=th)for(uint32_t x=0;x<w;x+=tw){if(TIFFReadTile(t,tile,x,y,0,s)<0)return 5;for(uint32_t yy=0;yy<th&&y+yy<h;yy++)for(uint32_t xx=0;xx<tw&&x+xx<w;xx++){if(planar==PLANARCONFIG_SEPARATE)memcpy(out+(y+yy)*rowbytes+((x+xx)*spp+s)*sample,tile+(yy*tw+xx)*sample,sample);else memcpy(out+(y+yy)*rowbytes+(x+xx)*spp*sample,tile+(yy*tw+xx)*spp*sample,spp*sample);}}free(tile);
 }else{tmsize_t linesize=TIFFScanlineSize(t);if(linesize<=0||linesize>640000000)return 4;unsigned char*line=malloc(linesize);if(!line)return 4;for(uint16_t s=0;s<(planar==PLANARCONFIG_SEPARATE?spp:1);s++)for(uint32_t y=0;y<h;y++){if(TIFFReadScanline(t,line,y,s)<0)return 5;if(planar==PLANARCONFIG_SEPARATE){for(uint32_t x=0;x<w;x++)memcpy(out+y*rowbytes+(x*spp+s)*sample,line+x*sample,sample);}else memcpy(out+y*rowbytes,line,rowbytes);}free(line);}
 uint16_t extras=0,*extra=NULL;TIFFGetField(t,TIFFTAG_EXTRASAMPLES,&extras,&extra);uint32_t associated=format|(extras&&extra[0]==EXTRASAMPLE_ASSOCALPHA?0x10000:0);
 u32(w);u32(h);u32(photo);u32(bits);u32(spp);u32(associated);u32(orientation);u32(plen);fwrite(profile,1,plen,stdout);fwrite(out,1,total,stdout);free(out);TIFFClose(t);free(m.data);return 0;
}
