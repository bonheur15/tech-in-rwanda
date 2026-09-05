import Image from '@tiptap/extension-image';
import Link from '@tiptap/extension-link';
import Placeholder from '@tiptap/extension-placeholder';
import StarterKit from '@tiptap/starter-kit';

export type ImagePlacement = 'small' | 'center' | 'wide' | 'full' | 'left' | 'right';
export type CropAspect = 'original' | '16:9' | '4:3' | '1:1';
export interface ImageNodeAttributes {
  assetId: string;
  src: string;
  alt: string;
  caption: string;
  credit: string;
  placement: ImagePlacement;
  width: number;
  cropAspect: CropAspect;
  focalX: number;
  focalY: number;
}

export const EditorialImage = Image.extend({
  addAttributes() {
    return {
      ...this.parent?.(),
      assetId: { default: '' },
      caption: { default: '' },
      credit: { default: '' },
      placement: { default: 'center' },
      width: { default: 100 },
      cropAspect: { default: 'original' },
      focalX: { default: 0.5 },
      focalY: { default: 0.5 },
    };
  },
});

export const editorExtensions = [
  StarterKit.configure({ heading: { levels: [2, 3] }, link: false }),
  Link.configure({
    openOnClick: false,
    autolink: true,
    defaultProtocol: 'https',
    protocols: ['http', 'https', 'mailto'],
    HTMLAttributes: { rel: 'noopener noreferrer' },
  }),
  Placeholder.configure({ placeholder: 'Begin writing…' }),
  EditorialImage.configure({ allowBase64: false }),
];

export function safeLink(value: string) {
  const href = value.trim();
  return /^(https?:\/\/|mailto:|\/|#)/i.test(href) ? href : href ? `https://${href}` : '';
}

export function clampImageWidth(placement: ImagePlacement, width: number) {
  const maximum = placement === 'left' || placement === 'right' ? 60 : 100;
  return Math.max(30, Math.min(maximum, Math.round(width / 5) * 5));
}
