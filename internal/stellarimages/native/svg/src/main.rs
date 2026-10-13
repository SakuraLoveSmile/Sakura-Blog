use std::io::{Read, Write};

fn main() {
    if let Err(error) = render() { eprintln!("{error}"); std::process::exit(1); }
}
fn render() -> Result<(), Box<dyn std::error::Error>> {
    let mut input = Vec::new();
    std::io::stdin().take(32 * 1024 * 1024 + 1).read_to_end(&mut input)?;
    if input.len() > 32 * 1024 * 1024 { return Err("SVG exceeds download limit".into()); }
    let mut options = resvg::usvg::Options::default();
    // 不提供宿主文件或 HTTP 目录；WASI 没有预开放目录。
    options.resources_dir = None;
    options.dpi = 72.0;
    options.font_family = "Noto Sans".into();
    options.fontdb_mut().load_font_data(include_bytes!("../NotoSans.ttf").to_vec());
    options.fontdb_mut().load_font_data(include_bytes!("../NotoSansSC.ttf").to_vec());
    options.fontdb_mut().set_sans_serif_family("Noto Sans");
    options.fontdb_mut().set_serif_family("Noto Sans");
    options.fontdb_mut().set_monospace_family("Noto Sans");
    let tree = resvg::usvg::Tree::from_data(&input, &options)?;
    let size = tree.size().to_int_size();
    if size.width() as u64 * size.height() as u64 > 40000000 { return Err("SVG exceeds pixel limit".into()); }
    let scale = (64.0 / size.width().max(size.height()) as f32).min(1.0);
    let width = (size.width() as f32 * scale).round().max(1.0) as u32;
    let height = (size.height() as f32 * scale).round().max(1.0) as u32;
    let mut pixmap = resvg::tiny_skia::Pixmap::new(width, height).ok_or("invalid SVG dimensions")?;
    resvg::render(&tree, resvg::tiny_skia::Transform::from_scale(scale,scale), &mut pixmap.as_mut());
    let mut output = std::io::stdout().lock();
    output.write_all(&size.width().to_le_bytes())?;
    output.write_all(&size.height().to_le_bytes())?;
    output.write_all(&width.to_le_bytes())?;
    output.write_all(&height.to_le_bytes())?;
    // tiny-skia 的像素已预乘，Go RGBA 使用相同约定。
    output.write_all(pixmap.data())?;
    Ok(())
}
