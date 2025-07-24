export async function compressImage(file: File, quality = 0.8, maxDim = 1920): Promise<File> {
  if (!file.type.startsWith('image/')) {
    return file;
  }
  return new Promise((resolve, reject) => {
    const img = new Image();
    img.onload = () => {
      let width = img.width;
      let height = img.height;
      if (width > maxDim || height > maxDim) {
        const ratio = Math.min(maxDim / width, maxDim / height);
        width = Math.round(width * ratio);
        height = Math.round(height * ratio);
      }
      const canvas = document.createElement('canvas');
      canvas.width = width;
      canvas.height = height;
      const ctx = canvas.getContext('2d');
      if (!ctx) {
        resolve(file);
        return;
      }
      ctx.drawImage(img, 0, 0, width, height);
      canvas.toBlob(blob => {
        if (!blob) {
          resolve(file);
          return;
        }
        const newFile = new File([blob], file.name, { type: file.type });
        resolve(newFile);
      }, file.type === 'image/png' ? 'image/png' : 'image/jpeg', quality);
    };
    img.onerror = () => resolve(file);
    img.src = URL.createObjectURL(file);
  });
}

