use std::io::{Read, Write};

// 与 libvips SVG loader 相同：百分比尺寸先用 viewBox，最后用可见几何。
fn pixels(length: rsvg::Length) -> f64 {
    use rsvg::LengthUnit::*;
    length.length * match length.unit {
        Px => 1.0, Em => 12.0, Ex => 6.0, In => 72.0,
        Cm => 72.0 / 2.54, Mm => 72.0 / 25.4, Pt => 1.0, Pc => 12.0,
        Percent => 0.0, _ => 0.0,
    }
}

fn natural_size(renderer: &rsvg::CairoRenderer<'_>) -> Result<(f64, f64), rsvg::RenderingError> {
    if let Some(size) = renderer.intrinsic_size_in_pixels() { return Ok(size); }
    let dimensions = renderer.intrinsic_dimensions();
    let (mut width, mut height) = (pixels(dimensions.width), pixels(dimensions.height));
    if let Some(viewbox) = dimensions.vbox {
        if width > 0.0 && height <= 0.0 { height = width * viewbox.height() / viewbox.width(); }
        else if height > 0.0 && width <= 0.0 { width = height * viewbox.width() / viewbox.height(); }
        else if width <= 0.0 && height <= 0.0 { width = viewbox.width(); height = viewbox.height(); }
    }
    if width <= 0.0 || height <= 0.0 {
        let (ink, _) = renderer.geometry_for_element(None)?;
        width = ink.x() + ink.width();
        height = ink.y() + ink.height();
    }
    Ok((width, height))
}

fn main() {
    if let Err(error) = render() {
        eprintln!("{error}");
        std::process::exit(1);
    }
}

fn render() -> Result<(), Box<dyn std::error::Error>> {
    let mut input = Vec::new();
    std::io::stdin().take(32 * 1024 * 1024 + 1).read_to_end(&mut input)?;
    if input.len() > 32 * 1024 * 1024 { return Err("SVG exceeds download limit".into()); }
    let bytes = glib::Bytes::from_owned(input);
    let stream = gio::MemoryInputStream::from_bytes(&bytes);
    let handle = rsvg::Loader::new().read_stream(&stream, None::<&gio::File>, None::<&gio::Cancellable>)?;
    let renderer = rsvg::CairoRenderer::new(&handle).with_dpi(72.0, 72.0);
    let (width, height) = natural_size(&renderer)?;
    if !width.is_finite() || !height.is_finite() || width <= 0.0 || height <= 0.0 {
        return Err("invalid SVG dimensions".into());
    }
    let width = width.ceil() as u32;
    let height = height.ceil() as u32;
    if width == 0 || height == 0 || u64::from(width) * u64::from(height) > 40000000 { return Err("invalid SVG dimensions".into()); }
    let scale = (64.0 / f64::from(width.max(height))).min(1.0);
    let out_width = (f64::from(width) * scale).round().max(1.0) as u32;
    let out_height = (f64::from(height) * scale).round().max(1.0) as u32;
    let mut surface = cairo::ImageSurface::create(cairo::Format::ARgb32, out_width as i32, out_height as i32)?;
    {
        let context = cairo::Context::new(&surface)?;
        renderer.render_document(&context, &cairo::Rectangle::new(0.0, 0.0, f64::from(out_width), f64::from(out_height)))?;
    }
    surface.flush();
    let stride = surface.stride() as usize;
    let pixels = surface.data()?;
    let mut output = std::io::stdout().lock();
    for value in [width, height, out_width, out_height] { output.write_all(&value.to_le_bytes())?; }
    // Cairo ARGB32 在小端 WASI 内存中为 BGRA，保持预乘 alpha。
    for row in pixels.chunks(stride).take(out_height as usize) {
        for pixel in row[..out_width as usize * 4].chunks_exact(4) {
            output.write_all(&[pixel[2], pixel[1], pixel[0], pixel[3]])?;
        }
    }
    Ok(())
}
