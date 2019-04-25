+++
# Hero widget.
widget = "hero"  # Do not modify this line!
active = true  # Activate this widget? true/false
weight = 1  # Order that this section will appear.

title = "KubeFed"

# Hero image (optional). Enter filename of an image in the `static/img/` folder.
#hero_media = ""

[design.background]
  # Apply a background color, gradient, or image.
  #   Uncomment (by removing `#`) an option to apply it.
  #   Choose a light or dark text color by setting `text_color_light`.
  #   Any HTML color name or Hex value is valid.

  # Background color.
  color = "#000"
  
  # Background gradient.
  # gradient_start = "#4bb4e3"
  # gradient_end = "#2b94c3"
  
  # Background image.
   image = "headers/bubbles-wide.jpg"  # Name of image in `static/img/`.
   image_darken = 0.6  # Darken the image? Range 0-1 where 0 is transparent and 1 is opaque.

  # Text color (true=light or false=dark).
  text_color_light = true

# Call to action links (optional).
#   Display link(s) by specifying a URL and label below. Icon is optional for `[cta]`.
#   Remove a link/note by deleting a cta/note block.
[cta]
  url = "post/getting-started/"
  label = "Get Started"
  icon_pack = "fas"
  icon = "download"
  
[cta_alt]
  url = "https://sourcethemes.com/academic/"
  label = "View Documentation"

# Note. An optional note to show underneath the links.
[cta_note]
  label = '<a id="academic-release" href="https://github.com/kubernetes-sigs/federation-v2" data-repo="kubernetes-sigs/federation-v2">Latest release <!-- V --></a>'
+++
KubeFed defines the APIs and API groups which encompass basic tenets needed to federate any given Kubernetes resource.

<span style="text-shadow: none;"><a class="github-button" href="https://github.com/kubernetes-sigs/federation-v2" data-icon="octicon-star" data-size="large" data-show-count="true" aria-label="Star this on GitHub">Star</a><script async defer src="https://buttons.github.io/buttons.js"></script></span>
