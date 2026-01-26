// Rust FFI bindings for the Go C library
//
// Build the Go library first:
//   make lib
//
// Compile this example:
//   rustc --edition 2021 -L ./apps/garbled -l garbled rust_ffi_example.rs
//
// Run with library path:
//   LD_LIBRARY_PATH=./apps/garbled ./rust_ffi_example
//
// Or set the circuit directory via environment variable:
//   MPC_CIRC_DIR=/path/to/circuit/dir LD_LIBRARY_PATH=./apps/garbled ./rust_ffi_example

use std::ffi::CString;
use std::os::raw::{c_char, c_int, c_uchar};
use std::ptr;

#[link(name = "garbled")]
extern "C" {
    /// Evaluator function
    /// Returns 0 on success, -1 on error
    fn c_evaluator_fn(
        circ_dir: *const c_char,
        sid: *const c_char,
        ui: *const c_char,
        result_ptr: *mut *mut c_uchar,
        result_len: *mut c_int,
    ) -> c_int;

    /// Garbler function
    /// Returns 0 on success, -1 on error
    fn c_garbler_fn(
        circ_dir: *const c_char,
        session_id: *const c_char,
        ui: *const c_char,
        cc: *const c_char,
        cnum: *const c_char,
        result_ptr: *mut *mut c_uchar,
        result_len: *mut c_int,
    ) -> c_int;

    /// Free memory allocated by C functions
    fn c_free_result(ptr: *mut c_uchar);
}

/// Safe Rust wrapper for evaluator function
pub fn evaluator(circ_dir: &str, session_id: &str, ui: &str) -> Result<Vec<u8>, String> {
    let c_circ_dir = CString::new(circ_dir).map_err(|e| e.to_string())?;
    let c_sid = CString::new(session_id).map_err(|e| e.to_string())?;
    let c_ui = CString::new(ui).map_err(|e| e.to_string())?;

    let mut result_ptr: *mut c_uchar = ptr::null_mut();
    let mut result_len: c_int = 0;

    let ret = unsafe {
        c_evaluator_fn(
            c_circ_dir.as_ptr(),
            c_sid.as_ptr(),
            c_ui.as_ptr(),
            &mut result_ptr,
            &mut result_len,
        )
    };

    if ret != 0 {
        return Err("c_evaluator_fn failed".to_string());
    }

    if result_ptr.is_null() {
        return Err("result pointer is null".to_string());
    }

    let result = unsafe {
        let slice = std::slice::from_raw_parts(result_ptr, result_len as usize);
        let vec = slice.to_vec();
        c_free_result(result_ptr);
        vec
    };

    Ok(result)
}

/// Safe Rust wrapper for garbler function
pub fn garbler(
    circ_dir: &str,
    session_id: &str,
    ui: &str,
    cc: &str,
    cnum: &str,
) -> Result<Vec<u8>, String> {
    let c_circ_dir = CString::new(circ_dir).map_err(|e| e.to_string())?;
    let c_sid = CString::new(session_id).map_err(|e| e.to_string())?;
    let c_ui = CString::new(ui).map_err(|e| e.to_string())?;
    let c_cc = CString::new(cc).map_err(|e| e.to_string())?;
    let c_cnum = CString::new(cnum).map_err(|e| e.to_string())?;

    let mut result_ptr: *mut c_uchar = ptr::null_mut();
    let mut result_len: c_int = 0;

    let ret = unsafe {
        c_garbler_fn(
            c_circ_dir.as_ptr(),
            c_sid.as_ptr(),
            c_ui.as_ptr(),
            c_cc.as_ptr(),
            c_cnum.as_ptr(),
            &mut result_ptr,
            &mut result_len,
        )
    };

    if ret != 0 {
        return Err("c_garbler_fn failed".to_string());
    }

    if result_ptr.is_null() {
        return Err("result pointer is null".to_string());
    }

    let result = unsafe {
        let slice = std::slice::from_raw_parts(result_ptr, result_len as usize);
        let vec = slice.to_vec();
        c_free_result(result_ptr);
        vec
    };

    Ok(result)
}

fn main() {
    println!("Testing Rust FFI bindings for Go C library\n");

    // Get the circuit directory path
    // You can pass this as a command line argument or environment variable
    let circ_dir = std::env::var("MPC_CIRC_DIR")
        .unwrap_or_else(|_| "./apps/garbled/circ_dir".to_string());

    println!("Using circuit directory: {}\n", circ_dir);

    // Test evaluator
    println!("Testing evaluator function...");
    match evaluator(&circ_dir, "test_session_1", "0x1919810") {
        Ok(result) => {
            println!("Evaluator result: {}", hex::encode(&result));
        }
        Err(e) => {
            eprintln!("Evaluator error: {}", e);
        }
    }

    // Test garbler
    println!("\nTesting garbler function...");
    match garbler(
        &circ_dir,
        "test_session_2",
        "0x114514",
        "0x4de216d2fdc9301e5b9c78486f7109a05670d200d9e2f275ec0aad08ec42afe7",
        "893",
    ) {
        Ok(result) => {
            println!("Garbler result: {}", hex::encode(&result));
        }
        Err(e) => {
            eprintln!("Garbler error: {}", e);
        }
    }
}

// Helper module for hex encoding (you can use the `hex` crate instead)
mod hex {
    pub fn encode(bytes: &[u8]) -> String {
        bytes.iter()
            .map(|b| format!("{:02x}", b))
            .collect::<String>()
    }
}
