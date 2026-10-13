/* LittleCMS 2.17 桥接，MIT。stdin：小端 profile-size/model/pixels，随后
 * profile 及打包像素；stdout：小端 RGB16 像素。 */
#include <lcms2.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include "sRGB_profile.h"
#include "p3_profile.h"

static uint32_t read_u32(void) { unsigned char b[4]; if (fread(b,1,4,stdin)!=4) exit(2); return b[0]|b[1]<<8|b[2]<<16|b[3]<<24; }
int main(void) {
 uint32_t profile_size=read_u32(),options=read_u32(),count=read_u32(),model=options&255;
 if(profile_size>32*1024*1024 || count>40000000 || model<1 || model>8) return 2;
 unsigned formats[]={0,TYPE_RGB_8,TYPE_RGB_16,TYPE_CMYK_8,TYPE_CMYK_16,TYPE_Lab_8,TYPE_Lab_16,TYPE_GRAY_8,TYPE_GRAY_16};
 unsigned channels[]={0,3,3,4,4,3,3,1,1}; unsigned bytes=(model%2==0)?2:1;
 unsigned char *profile=malloc(profile_size),*input=malloc((size_t)count*channels[model]*bytes);
 uint16_t *output=malloc((size_t)count*3*2);
 if((profile_size&&!profile)||!input||!output) return 2;
 if(fread(profile,1,profile_size,stdin)!=profile_size||fread(input,channels[model]*bytes,count,stdin)!=count) return 2;
 cmsHPROFILE source=profile_size?cmsOpenProfileFromMem(profile,profile_size):NULL;
 if(!source) { if(model==5||model==6) source=cmsCreateLab4Profile(NULL); else if(model==1||model==2) source=cmsCreate_sRGBProfile(); }
 cmsHPROFILE target=options&256?cmsOpenProfileFromMem(profile_p3,sizeof(profile_p3)):cmsOpenProfileFromMem(profile_sRGB,sizeof(profile_sRGB)); if(!source||!target) {fprintf(stderr,"invalid color profile\n");return 3;}
 int eight=model%2||options&512;cmsHTRANSFORM transform=cmsCreateTransform(source,formats[model],target,eight?TYPE_RGB_8:TYPE_RGB_16,INTENT_PERCEPTUAL,0);
 if(!transform) {fprintf(stderr,"cannot create color transform\n");return 3;}
 cmsDoTransform(transform,input,output,count);if(eight){unsigned char*p=(unsigned char*)output;for(size_t i=(size_t)count*3;i>0;i--)output[i-1]=p[i-1]*257;} if(fwrite(output,6,count,stdout)!=count) return 2;
 cmsDeleteTransform(transform);cmsCloseProfile(source);cmsCloseProfile(target);free(profile);free(input);free(output);return 0;
}
