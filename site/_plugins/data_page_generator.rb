# coding: utf-8
# Generate pages from individual records in yml files
# (c) 2014-2016 Adolfo Villafiorita
# Distributed under the conditions of the MIT License

module Jekyll

  module Sanitizer
    # strip characters and whitespace to create valid filenames, also lowercase
    def sanitize_filename(name)
      if(name.is_a? Integer)
        return name.to_s
      end
      return name.tr(
  "ÀÁÂÃÄÅàáâãäåĀāĂăĄąÇçĆćĈĉĊċČčÐðĎďĐđÈÉÊËèéêëĒēĔĕĖėĘęĚěĜĝĞğĠġĢģĤĥĦħÌÍÎÏìíîïĨĩĪīĬĭĮįİıĴĵĶķĸĹĺĻļĽľĿŀŁłÑñŃńŅņŇňŉŊŋÑñÒÓÔÕÖØòóôõöøŌōŎŏŐőŔŕŖŗŘřŚśŜŝŞşŠšſŢţŤťŦŧÙÚÛÜùúûüŨũŪūŬŭŮůŰűŲųŴŵÝýÿŶŷŸŹźŻżŽž",
  "AAAAAAaaaaaaAaAaAaCcCcCcCcCcDdDdDdEEEEeeeeEeEeEeEeEeGgGgGgGgHhHhIIIIiiiiIiIiIiIiIiJjKkkLlLlLlLlLlNnNnNnNnnNnNnOOOOOOooooooOoOoOoRrRrRrSsSsSsSssTtTtTtUUUUuuuuUuUuUuUuUuUuWwYyyYyYZzZzZz"
).downcase.strip.gsub(' ', '-').gsub(/[^\w.-]/, '')
    end
  end

  # this class is used to tell Jekyll to generate a page
  class DataPage < Page
    include Sanitizer

    # - site and base are copied from other plugins: to be honest, I am not sure what they do
    #
    # - `index_files` specifies if we want to generate named folders (true) or not (false)
    # - `dir` is the default output directory
    # - `data` is the data defined in `_data.yml` of the record for which we are generating a page
    # - `name` is the key in `data` which determines the output filename
    # - `template` is the name of the template for generating the page
    # - `extension` is the extension for the generated file
    def initialize(site, base, index_files, dir, data, name, template, extension)
      @site = site
      @base = base

      # @dir is the directory where we want to output the page
      # @name is the name of the page to generate
      #
      # the value of these variables changes according to whether we
      # want to generate named folders or not
      if data[name] == nil
        puts "error (datapage_gen). empty value for field '#{name}' in record #{data}"
      else
        filename = sanitize_filename(data[name]).to_s

        @dir = dir + (index_files ? "/" + filename + "/" : "")
        @name = (index_files ? "index" : filename) + "." + extension.to_s

        self.process(@name)
        self.read_yaml(File.join(base, '_layouts'), template + ".html")
        self.data['title'] = data[name]
        # add all the information defined in _data for the current record to the
        # current page (so that we can access it with liquid tags)
        self.data.merge!(data)
      end
    end
  end

  class DataPagesGenerator < Generator
    safe true

    # generate loops over _config.yml/page_gen invoking the DataPage
    # constructor for each record for which we want to generate a page

    def generate(site)
      # page_gen_dirs determines whether we want to generate index pages
      # (name/index.html) or standard files (name.html). This information
      # is passed to the DataPage constructor, which sets the @dir variable
      # as required by this directive
      index_files = site.config['page_gen-dirs'] == true

      # data contains the specification of the data for which we want to generate
      # the pages (look at the README file for its specification)
      data = site.config['page_gen']
      if data
        data.each do |data_spec|
          index_files_for_this_data = data_spec['index_files'] != nil ? data_spec['index_files'] : index_files
          template = data_spec['template'] || data_spec['data']
          name = data_spec['name']
          dir = data_spec['dir'] || data_spec['data']
          extension = data_spec['extension'] || "html"

          if site.layouts.key? template
            # records is the list of records defined in _data.yml
            # for which we want to generate different pages
            records = nil
            data_spec['data'].split('.').each do |level|
              if records.nil?
                records = site.data[level]
              else
                records = records[level]
              end
            end

            # apply filtering conditions:
            # - filter requires the name of a boolean field
            # - filter_condition evals a ruby expression
            records = records.select { |r| r[data_spec['filter']] } if data_spec['filter']
            records = records.select { |record| eval(data_spec['filter_condition']) } if data_spec['filter_condition']

            records.each do |record|
              site.pages << DataPage.new(site, site.source, index_files_for_this_data, dir, record, name, template, extension)
            end
          else
            puts "error (datapage_gen). could not find template #{template}" if not site.layouts.key? template
          end
        end
      end
    end
  end

  module DataPageLinkGenerator
    include Sanitizer

    # use it like this: {{input | datapage_url: dir}}
    # to generate a link to a data_page.
    #
    # the filter is smart enough to generate different link styles
    # according to the data_page-dirs directive ...
    #
    # ... however, the filter is not smart enough to support different
    # extensions for filenames.
    #
    # Thus, if you use the `extension` feature of this plugin, you
    # need to generate the links by hand
    def datapage_url(input, dir)
      extension = Jekyll.configuration({})['page_gen-dirs'] ? '/' : '.html'
      "#{dir}/#{sanitize_filename(input)}#{extension}"
    end
  end

end

Liquid::Template.register_filter(Jekyll::DataPageLinkGenerator)
