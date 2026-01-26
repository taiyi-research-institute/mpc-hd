# Rust FFI Bindings for MPC-HD

这个项目提供了 Rust FFI 绑定，用于调用 Go 实现的 `evaluator_fn` 和 `garbler_fn` 函数。

## 构建 C 共享库

首先需要构建 Go 代码生成的 C 共享库：

```bash
make lib
```

这将生成：
- `apps/garbled/libgarbled.so` - C 共享库
- `apps/garbled/libgarbled.h` - C 头文件

## C 函数接口

### c_evaluator_fn

```c
int c_evaluator_fn(
    const char* sid,          // Session ID
    const char* ui,           // Secret integer as "0x"-prefixed hex string (within 32 bytes)
    unsigned char** result_ptr, // Output: result byte array
    int* result_len           // Output: length of result
);
```

返回值：
- `0` 成功
- `-1` 失败

### c_garbler_fn

```c
int c_garbler_fn(
    const char* session_id,   // Session ID
    const char* ui,           // Secret integer as "0x"-prefixed hex string (within 32 bytes)
    const char* cc,           // Chain code as "0x"-prefixed hex string (within 32 bytes)
    const char* cnum,         // Chain number as decimal string
    unsigned char** result_ptr, // Output: result byte array
    int* result_len           // Output: length of result
);
```

返回值：
- `0` 成功
- `-1` 失败

### c_free_result

```c
void c_free_result(unsigned char* ptr);
```

释放由 C 函数分配的内存。

## Rust 使用示例

### 编译 Rust 示例

```bash
rustc --edition 2021 -L ./apps/garbled -l garbled rust_ffi_example.rs
```

### 运行示例

```bash
LD_LIBRARY_PATH=./apps/garbled ./rust_ffi_example
```

### 在 Rust 项目中使用

在你的 `Cargo.toml` 中添加：

```toml
[dependencies]
# 如果需要使用 hex crate
hex = "0.4"

[build-dependencies]
# 可选：使用 bindgen 自动生成绑定
# bindgen = "0.69"
```

创建 `build.rs`（可选）：

```rust
fn main() {
    println!("cargo:rustc-link-search=native=./apps/garbled");
    println!("cargo:rustc-link-lib=dylib=garbled");
    println!("cargo:rerun-if-changed=apps/garbled/libgarbled.so");
}
```

## Rust 代码示例

```rust
use std::ffi::CString;
use std::os::raw::{c_char, c_int, c_uchar};

// 调用 evaluator
let session_id = "你的session_id";
let ui = "0x1919810";
match evaluator(session_id, ui) {
    Ok(result) => println!("Result: {}", hex::encode(result)),
    Err(e) => eprintln!("Error: {}", e),
}

// 调用 garbler
let session_id = "你的session_id";
let ui = "0x114514";
let cc = "0x4de216d2fdc9301e5b9c78486f7109a05670d200d9e2f275ec0aad08ec42afe7";
let cnum = "893";
match garbler(session_id, ui, cc, cnum) {
    Ok(result) => println!("Result: {}", hex::encode(result)),
    Err(e) => eprintln!("Error: {}", e),
}
```

## 清理

删除生成的库文件：

```bash
make clean-lib
```

## 注意事项

1. **线程安全性**: Go 的运行时是多线程的，确保在多线程环境中正确使用
2. **内存管理**: 必须调用 `c_free_result` 释放返回的内存
3. **错误处理**: 函数返回 -1 时表示失败，详细错误信息会输出到日志
4. **环境变量**: 确保设置了正确的 `MPCLDIR` 环境变量（Go 代码会自动设置）
