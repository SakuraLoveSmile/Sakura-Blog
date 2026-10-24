#include <ffi.h>
#include <stdint.h>
#include <stdio.h>
static int32_t signed_sum(int8_t a,int16_t b,int32_t c){return a+b+c;}
static uint64_t wide_sum(uint64_t a,int64_t b){return a+b;}
static double floating_sum(float a,double b){return a+b;}
static void*same_pointer(void*p){return p;}
static int counter;static void increment(uint8_t n){counter+=n;}
int main(void){
 ffi_cif cif;ffi_type*types[3];void*args[3];int8_t a=-12;int16_t b=-340;int32_t c=900,out=0;types[0]=&ffi_type_sint8;types[1]=&ffi_type_sint16;types[2]=&ffi_type_sint32;args[0]=&a;args[1]=&b;args[2]=&c;
 if(ffi_prep_cif(&cif,FFI_DEFAULT_ABI,3,&ffi_type_sint32,types)!=FFI_OK)return 1;ffi_call(&cif,FFI_FN(signed_sum),&out,args);if(out!=548)return 2;
 uint64_t x=UINT64_C(0x100000000),wide=0;int64_t y=-3;types[0]=&ffi_type_uint64;types[1]=&ffi_type_sint64;args[0]=&x;args[1]=&y;
 if(ffi_prep_cif(&cif,FFI_DEFAULT_ABI,2,&ffi_type_uint64,types)!=FFI_OK)return 3;ffi_call(&cif,FFI_FN(wide_sum),&wide,args);if(wide!=UINT64_C(0xfffffffd))return 4;
 float f=1.25f;double d=2.125,total=0;types[0]=&ffi_type_float;types[1]=&ffi_type_double;args[0]=&f;args[1]=&d;
 if(ffi_prep_cif(&cif,FFI_DEFAULT_ABI,2,&ffi_type_double,types)!=FFI_OK)return 5;ffi_call(&cif,FFI_FN(floating_sum),&total,args);if(total!=3.375)return 6;
 void*p=&counter,*returned=NULL;types[0]=&ffi_type_pointer;args[0]=&p;if(ffi_prep_cif(&cif,FFI_DEFAULT_ABI,1,&ffi_type_pointer,types)!=FFI_OK)return 7;ffi_call(&cif,FFI_FN(same_pointer),&returned,args);if(returned!=p)return 8;
 uint8_t n=7;types[0]=&ffi_type_uint8;args[0]=&n;if(ffi_prep_cif(&cif,FFI_DEFAULT_ABI,1,&ffi_type_void,types)!=FFI_OK)return 9;ffi_call(&cif,FFI_FN(increment),NULL,args);if(counter!=7)return 10;
 puts("ffi scalar round-trip ok");return 0;
}
