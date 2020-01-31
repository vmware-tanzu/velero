# Title: YouTube plugin for Jekyll
# Description: Liquid tag to generate a YouTube embed.
# Authors:
#  - Joey Hoer (@joeyhoer | https://joeyhoer.com)
#
# Link: https://developers.google.com/youtube/player_parameters#Parameters
#
# Syntax: {% youtube [video_id] [width] [height] [query_param:value]... %}
#
# Examples:
#   {% youtube dQw4w9WgXcQ %}
#   {% youtube dQw4w9WgXcQ 600 rel:0 modestbranding:1 %}
#

module Jekyll
  class YouTube < Liquid::Tag

    ## Constants

    @@ATTRIBUTES = %w(
      autoplay
      cc_load_policy
      color
      controls
      disablekb
      enablejsapi
      end
      fs
      hl
      iv_load_policy
      list
      listType
      loop
      modestbranding
      origin
      playlist
      playsinline
      rel
      showinfo
      start
      widget_referrer
    )

    @ytid = nil
    @width = ''
    @height = ''

    def initialize(tag_name, markup, tokens)
      @content=markup

      @config = {}

      # Override configuration with values defined within _config.yml
      if Jekyll.configuration({}).has_key?('youtube')
        config = Jekyll.configuration({})['youtube']
        override_config(config)
      end

      params = markup.split

      if params.shift =~ /(?:(?:https?:\/\/)?(?:www.)?(?:youtube.com\/(?:embed\/|watch\?v=)|youtu.be\/)?(\S+)(?:\?rel=\d)?)/i
        @video_id = $1
      end

      @width = (params[0].to_i > 0) ? params.shift.to_i : 560
      @height = (params[0].to_i > 0) ? params.shift.to_i : (@width / 16.0 * 9).ceil

      if params.size > 0
        # Override configuration with parameters defined within Liquid tag
        config = {} # Reset local config
        params.each do |param|
          param = param.gsub /\s+/, '' # Remove whitespaces
          key, value = param.split(':',2) # Split first occurrence of ':' only
          config["#{key}"] = value
        end
        override_config(config)
      end

      super
    end

    def override_config(config)
      config.each{ |key,value| @config[key] = value }
    end

    def render(context)
      ouptut = super

      if !@video_id
        @video_id = "#{context[@content.strip]}"
      end

      if @video_id
        template_path = File.join(Dir.pwd, "_includes", "youtube.html")
        if File.exist?(template_path)
          site = context.registers[:site]

          partial = File.read(template_path)
          template = Liquid::Template.parse(partial)

          template.render!(({"video_id" => @video_id, "width" => @width, "height" => @height, "query_string" => render_query_string()}).merge(site.site_payload))
        else
          "<iframe width=\"#{@width}\" height=\"#{@height}\" src=\"https://www.youtube.com/embed/#{@video_id}#{render_query_string()}\" frameborder=\"0\" allowfullscreen></iframe>"
        end
      else
        puts "YouTube Embed: Error processing input, expected syntax {% youtube video_id [width] [height] [data-attr:value] %}"
      end
    end

    def render_query_string
      result = []
      @config.each do |key,value|
        if @@ATTRIBUTES.include?(key.to_s)
          result << "#{key}=#{value}"
        end
      end
      return (!(result.empty?) ? '?' : '') + result.join('&')
    end

  end
end

Liquid::Template.register_tag('youtube', Jekyll::YouTube)
