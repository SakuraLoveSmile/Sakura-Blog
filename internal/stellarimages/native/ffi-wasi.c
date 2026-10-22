/* WASI 标量 ABI 桥接。GObject generic marshaller 只使用标量参数。
 * libffi 的通用 CIF/type 实现仍来自锁定的官方源码。 */
#include <ffi.h>
#include <ffi_common.h>
#include <stdint.h>
__attribute__((import_module("stellar_ffi"),import_name("call")))
void stellar_ffi_call(uint32_t cif,uint32_t function,uint32_t result,uint32_t arguments);
ffi_status FFI_HIDDEN ffi_prep_cif_machdep(ffi_cif *cif){
 if(cif->abi!=FFI_WASM32)return FFI_BAD_ABI;
 if(cif->nargs>1024||cif->rtype->type==FFI_TYPE_STRUCT||cif->rtype->type==FFI_TYPE_LONGDOUBLE||cif->rtype->type==FFI_TYPE_COMPLEX)return FFI_BAD_TYPEDEF;
 for(unsigned i=0;i<cif->nargs;i++)if(cif->arg_types[i]->type==FFI_TYPE_STRUCT||cif->arg_types[i]->type==FFI_TYPE_LONGDOUBLE||cif->arg_types[i]->type==FFI_TYPE_COMPLEX)return FFI_BAD_TYPEDEF;
 return FFI_OK;
}
ffi_status FFI_HIDDEN ffi_prep_cif_machdep_var(ffi_cif *cif,unsigned fixed,unsigned total){return fixed==total?ffi_prep_cif_machdep(cif):FFI_BAD_ABI;}
void ffi_call(ffi_cif*cif,void(*function)(void),void*result,void**arguments){stellar_ffi_call((uintptr_t)cif,(uintptr_t)function,(uintptr_t)result,(uintptr_t)arguments);}
